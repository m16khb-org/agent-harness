package agy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"issueops/internal/port"
)

const agyMCPCatalogSHA256Env = "ISSUEOPS_MCP_CATALOG_SHA256"

func writeAgyUserMCP(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "agy_user_mcp_config"}
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
	server, err := agyUserMCPServer(req)
	if err != nil {
		return file, err
	}
	servers["issueops"] = server
	return WriteJSONPlan(path, file.Kind, config, 0o600, req.DryRun)
}

func writeAgyProjectMCP(path, kind string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	config, err := agyProjectMCPConfig()
	if err != nil {
		return file, err
	}
	return WriteJSONPlan(path, kind, config, 0o644, dryRun)
}

func agyUserMCPServer(req port.NativeInstallRequest) (map[string]any, error) {
	return agyMCPServer(req.BinPath, req.Root)
}

func agyProjectMCPConfig() (map[string]any, error) {
	server, err := agyMCPServer("./bin/issueops", ".")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"mcpServers": map[string]any{
			"issueops_project": server,
		},
	}, nil
}

func agyMCPServer(command, root string) (map[string]any, error) {
	if MCPCatalogSHA256 == nil {
		return nil, fmt.Errorf("agy MCP catalog digest is not configured")
	}
	catalogSHA256, err := MCPCatalogSHA256()
	if err != nil {
		return nil, fmt.Errorf("compute agy MCP catalog digest: %w", err)
	}
	if catalogSHA256 == "" {
		return nil, fmt.Errorf("compute agy MCP catalog digest: empty digest")
	}
	return map[string]any{
		"command": command,
		"args":    []string{"mcp"},
		"env": map[string]any{
			"ISSUEOPS_ROOT":        root,
			agyMCPCatalogSHA256Env: catalogSHA256,
		},
	}, nil
}
