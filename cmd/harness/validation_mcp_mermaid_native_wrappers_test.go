package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationMCPMermaidNativeWrappersUseDefaultSurfaces(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeNativeIntegrationFixture(t, root, home)

	mcpBinary := writeMCPValidationFakeBinary(t, root)
	if step := validateMCP(mcpBinary, root); !step.OK || !strings.Contains(step.Stdout, "atomic_commit_preflight") {
		t.Fatalf("expected MCP wrapper success, got %+v", step)
	}

	writeFileForWrapperTest(t, filepath.Join(root, "GENIUS_THINK.md"), "# Diagram\n\n```mermaid\ngraph TD\n  A[\"OK\"]\n```\n")
	if issues := validateMermaidDocs(root); len(issues) != 0 {
		t.Fatalf("expected Mermaid wrapper success, got %v", issues)
	}

	if step := validateNativeIntegration(root); !step.OK || !strings.Contains(step.Stdout, "duplicate_mcp_warning_fixture") {
		t.Fatalf("expected native integration wrapper success, got %+v", step)
	}
}

func writeMCPValidationFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-harness")
	body := `#!/bin/sh
set -eu
case "$*" in
  "mcp")
    cat <<'EOF'
` + strings.TrimSuffix(validMCPResponses(), "\n") + `
EOF
    ;;
  "daemon stop --json")
    printf '{"ok":true}\n'
    ;;
  *)
    echo "unexpected fake harness args: $*" >&2
    exit 2
    ;;
esac
`
	writeFileForWrapperTest(t, path, body)
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeNativeIntegrationFixture(t *testing.T, root, home string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(root, "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "shared", "SKILL.md"),
	} {
		writeFileForWrapperTest(t, path, "ok\n")
	}
	writeFileForWrapperTest(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.agent_harness]\ncommand = \"agent-harness\"\n")
	writeFileForWrapperTest(t, filepath.Join(home, ".codex", "hooks.json"), `{"command":"agent-harness hook user-prompt"}`)
}

func writeFileForWrapperTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
