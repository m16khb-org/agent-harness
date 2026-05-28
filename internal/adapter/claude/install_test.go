package claude

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)

func TestClaudeInstallerDefaultsToUserScopeOnly(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	if !exists(filepath.Join(home, ".claude", "skills", "alpha")) {
		t.Fatalf("claude user skill link missing")
	}
	for _, path := range []string{filepath.Join(home, ".claude", "settings.json"), filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".mcp.json")} {
		if exists(path) {
			t.Fatalf("default installer wrote unexpected path %s", path)
		}
	}
}

func TestClaudeInstallerProjectLocalIsExplicit(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	req.ProjectLocal = true
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join(root, ".mcp.json")} {
		if !exists(path) {
			t.Fatalf("project-local installer did not write %s", path)
		}
	}
	if exists(filepath.Join(root, ".claude", "settings.json")) {
		t.Fatalf("project-local installer should not write Claude SessionStart settings")
	}
}
