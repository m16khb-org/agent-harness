package mcpcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatesMCPToolsRoundTrip(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	dir := t.TempDir()
	gatesFile := filepath.Join(dir, "GATES.md")

	initOutcome := handleGatesMCPToolCall(MCPToolCall{Name: "gates_init", Arguments: map[string]any{
		"file":  gatesFile,
		"scope": "mcp cycle",
		"gates": []any{"G1: printf gate | CHECK: printf mcp-ok | EXPECT: mcp-ok", "G2: manual outcome"},
	}})
	if initOutcome.IsError || initOutcome.Payload == nil {
		t.Fatalf("gates_init failed: %+v", initOutcome)
	}

	statusOutcome := handleGatesMCPToolCall(MCPToolCall{Name: "gates_status", Arguments: map[string]any{
		"workspace_root": dir, "cwd": dir, "files": []any{gatesFile},
	}})
	if statusOutcome.IsError {
		t.Fatalf("gates_status failed: %+v", statusOutcome)
	}

	checkOutcome := handleGatesMCPToolCall(MCPToolCall{Name: "gates_check", Arguments: map[string]any{
		"workspace_root": dir, "cwd": dir, "files": []any{gatesFile},
		"env_allowlist": []any{"PATH"},
	}})
	if checkOutcome.IsError {
		t.Fatalf("gates_check failed: %+v", checkOutcome)
	}
	updated, err := os.ReadFile(gatesFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "- [x] G1: printf gate") || !strings.Contains(string(updated), "EVIDENCE: mcp-ok") {
		t.Fatalf("gates_check did not update ledger:\n%s", string(updated))
	}

	abandonOutcome := handleGatesMCPToolCall(MCPToolCall{Name: "gates_abandon", Arguments: map[string]any{
		"file": gatesFile, "gate_id": "G2", "reason": "manual outcome verified in review",
	}})
	if abandonOutcome.IsError {
		t.Fatalf("gates_abandon failed: %+v", abandonOutcome)
	}

	reportOutcome := handleGatesMCPToolCall(MCPToolCall{Name: "gates_report", Arguments: map[string]any{
		"workspace_root": dir, "cwd": dir, "files": []any{gatesFile},
	}})
	if reportOutcome.IsError {
		t.Fatalf("gates_report failed: %+v", reportOutcome)
	}
}
