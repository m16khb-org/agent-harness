package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)

func TestVerifyActivationV1RejectsClaudeAliasAndStaleTarget(t *testing.T) {
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
	if _, err := VerifyActivationV1(req); err == nil {
		t.Fatal("obsolete Claude MCP alias was accepted")
	}
}
