package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

func VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	mcpPath := filepath.Join(req.Home, ".claude.json")
	raw, err := os.ReadFile(mcpPath)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Claude MCP readback has no mcpServers object")
	}
	if _, alias := servers["agent-harness"]; alias {
		return nil, fmt.Errorf("Claude MCP readback retained obsolete agent-harness alias")
	}
	actual, ok := servers["agent_harness"]
	if !ok {
		return nil, fmt.Errorf("Claude MCP readback has no agent_harness server")
	}
	actualDigest, err := installutil.SemanticSHA256(actual)
	if err != nil {
		return nil, err
	}
	expectedDigest, err := installutil.SemanticSHA256(claudeUserMCPServer(req))
	if err != nil {
		return nil, err
	}
	if actualDigest != expectedDigest {
		return nil, fmt.Errorf("Claude MCP readback does not target the canonical binary and HARNESS_ROOT")
	}
	hooksPath := filepath.Join(req.Home, ".claude", "settings.json")
	hooksDigest, err := installutil.VerifyHookActivation(hooksPath, claudeSettingsConfig(req.BinPath))
	if err != nil {
		return nil, fmt.Errorf("Claude hook readback failed: %w", err)
	}
	mcpEvidence, err := installutil.CaptureNativeActivationEvidence("claude", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installutil.CaptureNativeActivationEvidence("claude", "hooks", hooksPath, hooksDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
