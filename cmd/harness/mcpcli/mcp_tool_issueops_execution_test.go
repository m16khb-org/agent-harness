package mcpcli

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestExecutionActionRequestFromMCPPreservesAutoMode(t *testing.T) {
	wantAncestry := []model.NativeProcessReceipt{{
		PID: 42, StartedAt: "2026-07-22T00:00:00Z", Executable: "/usr/bin/codex",
	}}
	req := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "prepare", "id": "io-aaaaaaaaaaaa", "mode": "auto",
	}, wantAncestry)
	if req.Action != "prepare" || req.ID != "io-aaaaaaaaaaaa" || req.Mode != "auto" {
		t.Fatalf("MCP auto prepare request drifted: %#v", req)
	}
	if len(req.Actor.ProcessAncestry) != 1 || req.Actor.ProcessAncestry[0] != wantAncestry[0] {
		t.Fatalf("MCP execution adapter did not preserve observed process ancestry: %#v", req.Actor.ProcessAncestry)
	}
}

func TestHandleMCPIssueOpsExecutionPreservesResetRequiredFields(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if err := os.MkdirAll(filepath.Join(stateDir, "issueops"), 0o700); err != nil {
		t.Fatal(err)
	}
	outcome := handleMCPIssueOpsExecution(map[string]any{
		"action": "prepare", "id": "io-aaaaaaaaaaaa", "mode": "auto", "confirm": true,
	})
	if !outcome.Handled || !outcome.IsError {
		t.Fatalf("reset-required MCP mutation outcome = %#v", outcome)
	}
	payload, ok := outcome.Payload.(map[string]any)
	if !ok {
		t.Fatalf("reset-required MCP payload type = %T", outcome.Payload)
	}
	if payload["code"] != "reset_required" || payload["target_schema"] != 1 || payload["state_root"] != stateDir || payload["next_command"] == "" {
		t.Fatalf("reset-required MCP payload lost CLI parity fields: %#v", payload)
	}
}
