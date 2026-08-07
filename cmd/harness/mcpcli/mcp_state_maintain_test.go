package mcpcli

import (
	statestore "agent-harness/internal/adapter/outbound/state"
	statecontract "agent-harness/internal/contract/state"
	"testing"
)

func TestMCPStateMaintain(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	t.Setenv("HARNESS_WORKER_DIR", "")
	if _, err := statestore.StateWrite("mcp-maintain-smoke", "content"); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	outcome := handlePolicyStateMCPToolCall(MCPToolCall{Name: "state_maintain", Arguments: map[string]any{}})
	if outcome.Err != nil {
		t.Fatalf("state_maintain failed: %+v", outcome.Err)
	}
	result, ok := outcome.Payload.(statecontract.StateMaintainResult)
	if !ok {
		t.Fatalf("unexpected payload type %T", outcome.Payload)
	}
	if !result.OK || len(result.Roots) != 1 || len(result.Skipped) != 3 {
		t.Fatalf("expected 1 maintained root and 3 skipped, got %+v", result)
	}
}
