package claude

import (
	"encoding/json"
	"os"
	"strings"

	"agent-harness/internal/port"
)

func writeClaudeUserMCP(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "claude_user_mcp_config"}
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return file, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	mcpServers, _ := config["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = map[string]any{}
		config["mcpServers"] = mcpServers
	}
	delete(mcpServers, "agent-harness")
	mcpServers["agent_harness"] = claudeUserMCPServer(req)
	return WriteJSONPlan(path, file.Kind, config, 0o600, req.DryRun)
}

func claudeUserMCPServer(req port.NativeInstallRequest) map[string]any {
	return map[string]any{
		"type":    "stdio",
		"command": req.BinPath,
		"args":    []string{"mcp"},
		"env": map[string]any{
			"HARNESS_ROOT": req.Root,
		},
	}
}
