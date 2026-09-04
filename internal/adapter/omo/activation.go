package omo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"issueops/internal/port"
)

func VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	omoRoot := filepath.Join(req.Home, ".omo")
	mcpPath := filepath.Join(omoRoot, "mcp.json")
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
		return nil, fmt.Errorf("Omo MCP readback has no mcpServers object")
	}
	actual, ok := servers["issueops"]
	if !ok {
		return nil, fmt.Errorf("Omo MCP readback has no issueops server")
	}
	actualDigest, err := SemanticSHA256(actual)
	if err != nil {
		return nil, err
	}
	expectedServer, err := omoUserMCPServer(req)
	if err != nil {
		return nil, err
	}
	expectedDigest, err := SemanticSHA256(expectedServer)
	if err != nil {
		return nil, err
	}
	if actualDigest != expectedDigest {
		return nil, fmt.Errorf("Omo MCP readback does not target the canonical binary and ISSUEOPS_ROOT")
	}

	extensionPath := filepath.Join(omoRoot, "extensions", "issueops.js")
	extension, err := os.ReadFile(extensionPath)
	if err != nil {
		return nil, err
	}
	expectedExtension := omoLifecycleExtension(req.BinPath)
	if string(extension) != expectedExtension {
		return nil, fmt.Errorf("Omo lifecycle extension does not match the canonical managed content")
	}
	extensionDigest, err := SemanticSHA256(map[string]any{
		"host": "omo", "surface": "hooks", "content": expectedExtension,
	})
	if err != nil {
		return nil, err
	}

	mcpEvidence, err := CaptureNativeActivationEvidence("omo", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := CaptureNativeActivationEvidence("omo", "hooks", extensionPath, extensionDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
