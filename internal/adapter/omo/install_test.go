package omo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/installutil"
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
}

func TestInstallerWritesNativeOmoSurfaces(t *testing.T) {
	req := omoTestRequest(t)
	req.ProjectLocal = true
	writeOmoTestJSON(t, filepath.Join(req.Home, ".omo", "mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			"other":         map[string]any{"command": "other"},
			"agent-harness": map[string]any{"command": "obsolete"},
		},
	})

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("Install returned error: %v\n%+v", err, result)
	}
	if !result.OK || result.Host != "omo" {
		t.Fatalf("unexpected install result: %+v", result)
	}

	assertOmoTestSkillLink(t, filepath.Join(req.Home, ".omo", "skills", "alpha"), filepath.Join(req.Root, "skills", "alpha"))
	assertOmoTestSkillLink(t, filepath.Join(req.Root, ".omo", "skills", "alpha"), filepath.Join(req.Root, "skills", "alpha"))

	globalMCP := readOmoTestJSON(t, filepath.Join(req.Home, ".omo", "mcp.json"))
	assertOmoTestMCPServer(t, globalMCP, "agent_harness", req.BinPath, req.Root)
	servers := globalMCP["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatal("Omo MCP merge removed unrelated server")
	}
	if _, ok := servers["agent-harness"]; ok {
		t.Fatal("Omo MCP merge retained obsolete alias")
	}

	projectMCP := readOmoTestJSON(t, filepath.Join(req.Root, ".omo", "mcp.json"))
	assertOmoTestMCPServer(t, projectMCP, "agent_harness_project", "./bin/agent-harness", ".")

	extension := readOmoTestFile(t, filepath.Join(req.Home, ".omo", "extensions", "agent-harness.js"))
	for _, token := range []string{
		`pi.on("session_start"`,
		`pi.on("session_compact"`,
		`event.accepted`,
		`"--json"`,
		`display: false`,
		req.BinPath,
	} {
		if !strings.Contains(extension, token) {
			t.Fatalf("Omo lifecycle extension missing %q:\n%s", token, extension)
		}
	}
	for _, path := range []string{
		filepath.Join(req.Root, "configs", "omo", "mcp.json"),
		filepath.Join(req.Root, "configs", "omo", "agent-harness.js"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing Omo template %s: %v", path, err)
		}
	}
}

func TestInstallerDryRunPlansWithoutWriting(t *testing.T) {
	req := omoTestRequest(t)
	req.ProjectLocal = true
	req.DryRun = true

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("dry-run returned error: %v\n%+v", err, result)
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(req.Home, ".omo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote Omo user directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(req.Root, ".omo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote project-local Omo directory: %v", err)
	}
	if !omoTestHasPlannedWrite(result.Files) || !omoTestHasPlannedLink(result.Links) {
		t.Fatalf("dry-run omitted planned files or links: %+v", result)
	}
}

func TestVerifyActivationRejectsTamperedExtension(t *testing.T) {
	req := omoTestRequest(t)
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	evidence, err := VerifyActivation(req)
	if err != nil {
		t.Fatalf("VerifyActivation returned error: %v", err)
	}
	if len(evidence) != 2 || evidence[0].Host != "omo" || evidence[0].Surface != "mcp" ||
		evidence[1].Host != "omo" || evidence[1].Surface != "hooks" {
		t.Fatalf("unexpected Omo activation evidence: %+v", evidence)
	}

	extensionPath := filepath.Join(req.Home, ".omo", "extensions", "agent-harness.js")
	if err := os.WriteFile(extensionPath, []byte("export default function () {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyActivation(req); err == nil || !strings.Contains(err.Error(), "lifecycle extension") {
		t.Fatalf("tampered extension must fail strict readback, got %v", err)
	}
}

func TestTrackedTemplatesMatchGeneratedContent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	extension := readOmoTestFile(t, filepath.Join(root, "configs", "omo", "agent-harness.js"))
	if extension != omoLifecycleExtension("./bin/agent-harness") {
		t.Fatal("tracked Omo lifecycle extension drifted from generated template")
	}
	config := readOmoTestJSON(t, filepath.Join(root, "configs", "omo", "mcp.json"))
	got, err := SemanticSHA256(config)
	if err != nil {
		t.Fatal(err)
	}
	want, err := SemanticSHA256(omoProjectMCPConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("tracked Omo MCP config drifted from generated template")
	}
}

func omoTestRequest(t *testing.T) port.NativeInstallRequest {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	skillDir := filepath.Join(root, "skills", "alpha")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: alpha\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return port.NativeInstallRequest{
		Root:       root,
		Home:       home,
		BinPath:    filepath.Join(root, "bin", "agent-harness"),
		SkillNames: []string{"alpha"},
	}
}

func writeOmoTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readOmoTestJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	body := readOmoTestFile(t, path)
	var value map[string]any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func readOmoTestFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func assertOmoTestMCPServer(t *testing.T, config map[string]any, name, command, root string) {
	t.Helper()
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers: %+v", config)
	}
	server, ok := servers[name].(map[string]any)
	if !ok {
		t.Fatalf("missing MCP server %q: %+v", name, servers)
	}
	if server["command"] != command {
		t.Fatalf("server %q command = %v, want %s", name, server["command"], command)
	}
	env, ok := server["env"].(map[string]any)
	if !ok || env["HARNESS_ROOT"] != root {
		t.Fatalf("server %q HARNESS_ROOT drifted: %+v", name, server)
	}
}

func assertOmoTestSkillLink(t *testing.T, path, target string) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve %s: %v", path, err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("resolve %s: %v", target, err)
	}
	if resolved != want {
		t.Fatalf("link %s resolves to %s, want %s", path, resolved, want)
	}
}

func omoTestHasPlannedWrite(files []port.InstallFile) bool {
	for _, file := range files {
		if file.WouldWrite {
			return true
		}
	}
	return false
}

func omoTestHasPlannedLink(links []port.InstallLink) bool {
	for _, link := range links {
		if link.WouldCreate {
			return true
		}
	}
	return false
}
