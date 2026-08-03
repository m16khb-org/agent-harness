package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestVerifyActivationRejectsClaudeAliasAndStaleTarget(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "agent-harness"))
	req.SkillNames = []string{"alpha"}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	evidence, err := VerifyActivation(req)
	if err != nil || len(evidence) != 2 {
		t.Fatalf("evidence=%#v err=%v", evidence, err)
	}
	path := filepath.Join(home, ".claude.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	servers := config["mcpServers"].(map[string]any)
	servers["agent-harness"] = servers["agent_harness"]
	changed, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyActivation(req); err == nil {
		t.Fatal("obsolete Claude MCP alias was accepted")
	}
}

func TestVerifyActivationRejectsClaudeWorktreeHookTarget(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "agent-harness"))
	req.SkillNames = []string{"alpha"}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(home, ".claude", "settings.json")
	raw, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(filepath.Dir(root), "source.worktrees", "completed", "bin", "agent-harness")
	raw = []byte(strings.ReplaceAll(string(raw), req.BinPath, stale))
	if err := os.WriteFile(hooksPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = VerifyActivation(req)
	if err == nil {
		t.Fatal("worktree Claude hook target was accepted")
	}
	for _, evidence := range []string{"Claude hook readback", "observed=" + stale, "expected=" + req.BinPath} {
		if !strings.Contains(err.Error(), evidence) {
			t.Fatalf("error = %q, want %q", err, evidence)
		}
	}
}
