package codex

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)

func TestVerifyActivationV1RejectsStaleCodexTarget(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "agent-harness"))
	req.SkillNames = []string{"alpha"}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	evidence, err := VerifyActivationV1(req)
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
	if _, err := VerifyActivationV1(req); err != nil {
		t.Fatalf("canonical semantic readback rejected formatting-only change: %v", err)
	}
	configPath := filepath.Join(req.CodexHome, "config.toml")
	if err := os.WriteFile(configPath, []byte("[mcp_servers.agent_harness]\ncommand = \"/old/bin\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyActivationV1(req); err == nil {
		t.Fatal("stale Codex MCP target was accepted")
	}
}
