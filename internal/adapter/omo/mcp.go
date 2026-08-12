package omo

import (
	"encoding/json"
	"os"
	"strings"

	"agent-harness/internal/port"
)

func writeOmoUserMCP(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "omo_user_mcp_config"}
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return file, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		config["mcpServers"] = servers
	}
	delete(servers, "agent-harness")
	servers["agent_harness"] = omoUserMCPServer(req)
	return WriteJSONPlan(path, file.Kind, config, 0o600, req.DryRun)
}

func omoUserMCPServer(req port.NativeInstallRequest) map[string]any {
	return omoMCPServer(req.BinPath, req.Root)
}

func omoProjectMCPConfig() map[string]any {
	return map[string]any{
		"mcpServers": map[string]any{
			"agent_harness_project": omoMCPServer("./bin/agent-harness", "."),
		},
	}
}

func omoMCPServer(command, root string) map[string]any {
	return map[string]any{
		"command": command,
		"args":    []string{"mcp"},
		"env": map[string]any{
			"HARNESS_ROOT": root,
		},
	}
}
