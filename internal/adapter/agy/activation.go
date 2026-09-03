package agy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/internal/port"
)

func VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	geminiRoot := filepath.Join(req.Home, ".gemini", "config")
	mcpPath := filepath.Join(geminiRoot, "mcp_config.json")
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
		return nil, fmt.Errorf("agy MCP readback has no mcpServers object")
	}
	if _, alias := servers["agent-harness"]; alias {
		return nil, fmt.Errorf("agy MCP readback retained obsolete agent-harness alias")
	}
	actual, ok := servers["agent_harness"]
	if !ok {
		return nil, fmt.Errorf("agy MCP readback has no agent_harness server")
	}
	actualDigest, err := SemanticSHA256(actual)
	if err != nil {
		return nil, err
	}
	expectedServer, err := agyUserMCPServer(req)
	if err != nil {
		return nil, err
	}
	expectedDigest, err := SemanticSHA256(expectedServer)
	if err != nil {
		return nil, err
	}
	if actualDigest != expectedDigest {
		return nil, fmt.Errorf("agy MCP readback does not target the canonical binary and HARNESS_ROOT")
	}

	mcpEvidence, err := CaptureNativeActivationEvidence("agy", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence}, nil
}
