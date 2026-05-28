package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestCodexInstallerWritesOnlyUserAndHarnessTemplatePaths(t *testing.T) {
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
	if _, err := os.Lstat(filepath.Join(home, ".codex", "skills", "alpha")); err != nil {
		t.Fatalf("codex user skill link missing: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "[mcp_servers.agent_harness]") || !strings.Contains(string(config), req.BinPath) {
		t.Fatalf("codex config missing harness block:\n%s", string(config))
	}
	hooks, err := os.ReadFile(filepath.Join(home, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hooks), "hook user-prompt") || !strings.Contains(string(hooks), req.BinPath) {
		t.Fatalf("codex hooks missing harness UserPromptSubmit hook:\n%s", string(hooks))
	}
	if exists(filepath.Join(root, ".claude", "skills", "alpha")) || exists(filepath.Join(root, ".mcp.json")) {
		t.Fatalf("codex installer wrote project-local files")
	}
}
