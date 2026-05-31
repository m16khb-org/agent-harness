package installutil

import (
	"os"
	"path/filepath"
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
