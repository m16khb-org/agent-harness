package agy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/installutil"
	mcpdomain "agent-harness/internal/domain/mcp"
	"agent-harness/internal/port"
)

func init() {
	NewInstallPlan = func(host string, dryRun bool) InstallPlan {
		return installutil.NewPlan(host, dryRun)
	}
	WriteJSONPlan = installutil.WriteJSONPlan
	WriteTextPlan = installutil.WriteTextPlan
	CaptureNativeActivationEvidence = installutil.CaptureNativeActivationEvidence
	EnsureSymlinkPlan = installutil.EnsureSymlinkPlan
	PlanHostSkillLinks = installutil.PlanHostSkillLinks
	SemanticSHA256 = installutil.SemanticSHA256
	MCPCatalogSHA256 = func() (string, error) {
		return SemanticSHA256(mcpdomain.AdvertisedTools())
	}
}

func TestInstallerWritesNativeAgySurfaces(t *testing.T) {
	req := agyTestRequest(t)
	req.ProjectLocal = true
	writeAgyTestJSON(t, filepath.Join(req.Home, ".gemini", "config", "mcp_config.json"), map[string]any{
		"mcpServers": map[string]any{
			"other":         map[string]any{"command": "other"},
			"agent-harness": map[string]any{"command": "obsolete"},
		},
	})

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("Install returned error: %v\n%+v", err, result)
	}
	if !result.OK || result.Host != "agy" {
		t.Fatalf("unexpected install result: %+v", result)
	}

	assertAgyTestSkillLink(t, filepath.Join(req.Home, ".gemini", "config", "skills", "alpha"), filepath.Join(req.Root, "skills", "alpha"))
	assertAgyTestSkillLink(t, filepath.Join(req.Root, ".agents", "skills", "alpha"), filepath.Join(req.Root, "skills", "alpha"))

	globalMCP := readAgyTestJSON(t, filepath.Join(req.Home, ".gemini", "config", "mcp_config.json"))
	assertAgyTestMCPServer(t, globalMCP, "agent_harness", req.BinPath, req.Root)
	servers := globalMCP["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatal("agy MCP merge removed unrelated server")
	}
	if _, ok := servers["agent-harness"]; ok {
		t.Fatal("agy MCP merge retained obsolete alias")
	}

	projectMCP := readAgyTestJSON(t, filepath.Join(req.Root, ".agents", "mcp_config.json"))
	assertAgyTestMCPServer(t, projectMCP, "agent_harness_project", "./bin/agent-harness", ".")

	templatePath := filepath.Join(req.Root, "configs", "agy", "mcp_config.json")
	if _, err := os.Stat(templatePath); err != nil {
		t.Fatalf("missing agy template %s: %v", templatePath, err)
	}
}

func TestInstallerDryRunPlansWithoutWriting(t *testing.T) {
	req := agyTestRequest(t)
	req.ProjectLocal = true
	req.DryRun = true

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("dry-run returned error: %v\n%+v", err, result)
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(req.Home, ".gemini")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote agy user directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(req.Root, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote project-local .agents directory: %v", err)
	}
	if !agyTestHasPlannedWrite(result.Files) || !agyTestHasPlannedLink(result.Links) {
		t.Fatalf("dry-run omitted planned files or links: %+v", result)
	}
}

func TestVerifyActivationRejectsTamperedMCP(t *testing.T) {
	req := agyTestRequest(t)
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	evidence, err := VerifyActivation(req)
	if err != nil {
		t.Fatalf("VerifyActivation returned error: %v", err)
	}
	if len(evidence) != 1 || evidence[0].Host != "agy" || evidence[0].Surface != "mcp" {
		t.Fatalf("unexpected agy activation evidence: %+v", evidence)
	}

	mcpPath := filepath.Join(req.Home, ".gemini", "config", "mcp_config.json")
	writeAgyTestJSON(t, mcpPath, map[string]any{
		"mcpServers": map[string]any{
			"agent_harness": map[string]any{"command": "tampered"},
		},
	})
	if _, err := VerifyActivation(req); err == nil || !strings.Contains(err.Error(), "canonical binary") {
		t.Fatalf("tampered MCP must fail strict readback, got %v", err)
	}
}

func agyTestRequest(t *testing.T) port.NativeInstallRequest {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	skillDir := filepath.Join(root, "skills", "alpha")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return port.NativeInstallRequest{
		Root:       root,
		Home:       home,
		BinPath:    filepath.Join(root, "bin", "agent-harness"),
		SkillNames: []string{"alpha"},
	}
}

func writeAgyTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readAgyTestJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}

func assertAgyTestSkillLink(t *testing.T, linkPath, target string) {
	t.Helper()
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("missing link %s: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at %s", linkPath)
	}
	actualTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink %s: %v", linkPath, err)
	}
	expectedTarget := target
	if !filepath.IsAbs(actualTarget) {
		expectedTarget, err = filepath.Rel(filepath.Dir(linkPath), target)
		if err != nil {
			t.Fatalf("rel target %s -> %s: %v", filepath.Dir(linkPath), target, err)
		}
	}
	if actualTarget != expectedTarget {
		t.Fatalf("link target mismatch for %s: got %s, want %s", linkPath, actualTarget, expectedTarget)
	}
}

func assertAgyTestMCPServer(t *testing.T, config map[string]any, serverName, command, root string) {
	t.Helper()
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("config missing mcpServers: %+v", config)
	}
	server, ok := servers[serverName].(map[string]any)
	if !ok {
		t.Fatalf("config missing server %s: %+v", serverName, servers)
	}
	if server["command"] != command {
		t.Fatalf("server command mismatch: got %v, want %v", server["command"], command)
	}
	env, ok := server["env"].(map[string]any)
	if !ok || env["HARNESS_ROOT"] != root {
		t.Fatalf("server HARNESS_ROOT mismatch: got %v, want %v", env, root)
	}
}

func agyTestHasPlannedWrite(files []port.InstallFile) bool {
	for _, file := range files {
		if file.WouldWrite {
			return true
		}
	}
	return false
}

func agyTestHasPlannedLink(links []port.InstallLink) bool {
	for _, link := range links {
		if link.WouldCreate {
			return true
		}
	}
	return false
}
