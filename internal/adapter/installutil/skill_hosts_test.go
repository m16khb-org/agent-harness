package installutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillEnabledForHostDefaultsToAllHosts(t *testing.T) {
	root := t.TempDir()
	if !SkillEnabledForHost(root, "missing", "codex") {
		t.Fatalf("missing install config should enable skill for codex")
	}
	if !SkillEnabledForHost(root, "missing", "claude") {
		t.Fatalf("missing install config should enable skill for claude")
	}
}

func TestSkillEnabledForHostHonorsHostList(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "codex-only")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install.json"), []byte(`{"hosts":["codex"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !SkillEnabledForHost(root, "codex-only", "codex") {
		t.Fatalf("codex should be enabled")
	}
	if SkillEnabledForHost(root, "codex-only", "claude") {
		t.Fatalf("claude should be disabled")
	}
}

func TestSkillEnabledForHostDefaultsToAllHostsForInvalidConfig(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "invalid")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install.json"), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !SkillEnabledForHost(root, "invalid", "codex") {
		t.Fatalf("invalid install config should keep historical default for codex")
	}
	if !SkillEnabledForHost(root, "invalid", "claude") {
		t.Fatalf("invalid install config should keep historical default for claude")
	}
}

func TestSkillNamesForHostPartitionsSkills(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"shared", "codex-only"} {
		if err := os.MkdirAll(filepath.Join(root, "skills", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "codex-only", "install.json"), []byte(`{"hosts":["codex"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	enabled, skipped := SkillNamesForHost(root, []string{"shared", "codex-only"}, "claude")
	if got, want := strings.Join(enabled, ","), "shared"; got != want {
		t.Fatalf("enabled skills = %q, want %q", got, want)
	}
	if got, want := strings.Join(skipped, ","), "codex-only"; got != want {
		t.Fatalf("skipped skills = %q, want %q", got, want)
	}
}
