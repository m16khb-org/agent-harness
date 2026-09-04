package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"issueops/internal/port"
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
	actual, ok := servers["issueops"]
	if !ok {
		return nil, fmt.Errorf("Claude MCP readback has no issueops server")
	}
	actualDigest, err := SemanticSHA256(actual)
	if err != nil {
		return nil, err
	}
	expectedDigest, err := SemanticSHA256(claudeUserMCPServer(req))
	if err != nil {
		return nil, err
	}
	if actualDigest != expectedDigest {
		return nil, fmt.Errorf("Claude MCP readback does not target the canonical binary and ISSUEOPS_ROOT")
	}
	hooksPath := filepath.Join(req.Home, ".claude", "settings.json")
	hooksDigest, err := VerifyHookActivation(hooksPath, claudeSettingsConfig(req.BinPath))
	if err != nil {
		return nil, fmt.Errorf("Claude hook readback failed: %w", err)
	}
	mcpEvidence, err := CaptureNativeActivationEvidence("claude", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := CaptureNativeActivationEvidence("claude", "hooks", hooksPath, hooksDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
