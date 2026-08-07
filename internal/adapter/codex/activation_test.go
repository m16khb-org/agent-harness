package codex

import (
	install "agent-harness/internal/adapter/install"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyActivationRejectsStaleCodexTarget(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "agent-harness"))
	req.SkillNames = []string{"alpha"}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	evidence, err := VerifyActivation(req)
	if err != nil || len(evidence) != 2 {
		t.Fatalf("evidence=%#v err=%v", evidence, err)
	}
	hooksPath := filepath.Join(req.CodexHome, "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hooksPath, append(data, []byte("\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	// Whitespace is raw evidence, not semantic drift; change the target itself.
	data, err = os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(string(data[:len(data)-1]))
	if err := os.WriteFile(hooksPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyActivation(req); err != nil {
		t.Fatalf("canonical semantic readback rejected formatting-only change: %v", err)
	}
	configPath := filepath.Join(req.CodexHome, "config.toml")
	if err := os.WriteFile(configPath, []byte("[mcp_servers.agent_harness]\ncommand = \"/old/bin\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyActivation(req); err == nil {
		t.Fatal("stale Codex MCP target was accepted")
	}
}

func TestVerifyActivationRejectsCodexWorktreeHookTarget(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "agent-harness"))
	req.SkillNames = []string{"alpha"}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(req.CodexHome, "hooks.json")
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
		t.Fatal("worktree Codex hook target was accepted")
	}
	for _, evidence := range []string{"Codex hook readback", "observed=" + stale, "expected=" + req.BinPath} {
		if !strings.Contains(err.Error(), evidence) {
			t.Fatalf("error = %q, want %q", err, evidence)
		}
	}
}
