package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

func VerifyActivationV1(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	configPath := filepath.Join(req.CodexHome, "config.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	text := string(config)
	expectedBlock := codexGlobalBlock(req)
	if strings.Count(text, "[mcp_servers.agent_harness]") != 1 || strings.Count(text, "[mcp_servers.agent_harness.env]") != 1 ||
		strings.Contains(text, "[mcp_servers.agent-harness]") || strings.Contains(text, "[mcp_servers.agent-harness.env]") ||
		!strings.HasSuffix(text, expectedBlock) {
		return nil, fmt.Errorf("Codex MCP readback does not contain exactly one canonical agent_harness server")
	}
	mcpDigest, err := installutil.SemanticSHA256(map[string]any{
		"host": "codex", "surface": "mcp", "block": expectedBlock,
	})
	if err != nil {
		return nil, err
	}
	hooksPath := filepath.Join(req.CodexHome, "hooks.json")
	hooksDigest, err := installutil.VerifyHookActivation(hooksPath, codexHooksConfig(req.BinPath))
	if err != nil {
		return nil, fmt.Errorf("Codex hook readback failed: %w", err)
	}
	mcpEvidence, err := installutil.CaptureNativeActivationEvidence("codex", "mcp", configPath, mcpDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installutil.CaptureNativeActivationEvidence("codex", "hooks", hooksPath, hooksDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
