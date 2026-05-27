package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestClaudeInstallerDefaultsToUserScopeOnly(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"), "~/wiki")
	req.SkillNames = []string{"alpha"}
	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	if _, err := os.Lstat(filepath.Join(home, ".claude", "skills", "alpha")); err != nil {
		t.Fatalf("claude user skill link missing: %v", err)
	}
	settings, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), "session-start-llm-wiki.sh") || !strings.Contains(string(settings), root) {
		t.Fatalf("claude user hook missing central command:\n%s", string(settings))
	}
	for _, path := range []string{filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".mcp.json")} {
		if exists(path) {
			t.Fatalf("default installer wrote project-local path %s", path)
		}
	}
}

func TestClaudeInstallerProjectLocalIsExplicit(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"), "~/wiki")
	req.SkillNames = []string{"alpha"}
	req.ProjectLocal = true
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".mcp.json")} {
		if !exists(path) {
			t.Fatalf("project-local installer did not write %s", path)
		}
	}
}
