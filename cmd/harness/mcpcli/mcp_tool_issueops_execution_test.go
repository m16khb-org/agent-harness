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
	req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "prepare", "id": "io-aaaaaaaaaaaa", "mode": "auto",
	}, wantAncestry)
	if err != nil {
		t.Fatal(err)
	}
	if req.Action != "prepare" || req.ID != "io-aaaaaaaaaaaa" || req.Mode != "auto" {
		t.Fatalf("MCP auto prepare request drifted: %#v", req)
	}
	if len(req.Actor.ProcessAncestry) != 1 || req.Actor.ProcessAncestry[0] != wantAncestry[0] {
		t.Fatalf("MCP execution adapter did not preserve observed process ancestry: %#v", req.Actor.ProcessAncestry)
	}
}

func TestExecutionActionRequestFromMCPIssueSnapshot(t *testing.T) {
	req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "prepare",
		"id":     "io-aaaaaaaaaaaa",
		"issue_snapshot": map[string]any{
			"provider": "gitlab",
			"source":   "glab_mcp",
			"web_url":  "https://gitlab.example.com/acme/repo/-/issues/69",
			"body":     "AC-69",
			"state":    "opened",
		},
	}, nil)
	if err != nil || req.IssueSnapshot == nil || req.IssueSnapshot.Source != "glab_mcp" {
		t.Fatalf("nested snapshot mapping failed: req=%#v err=%v", req, err)
	}
}

func TestExecutionActionRequestFromMCPRejectsMalformedIssueSnapshot(t *testing.T) {
	for name, snapshot := range map[string]any{
		"not_object": "glab_mcp",
		"non_string": map[string]any{
			"provider": "gitlab",
			"source":   "glab_mcp",
			"web_url":  69,
			"body":     "AC-69",
			"state":    "opened",
		},
		"unknown_field": map[string]any{
			"provider":         "gitlab",
			"source":           "glab_mcp",
			"web_url":          "https://gitlab.example.com/acme/repo/-/issues/69",
			"body":             "AC-69",
			"state":            "opened",
			"server_namespace": "private",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := executionActionRequestFromMCPWithAncestry(map[string]any{
				"action": "prepare", "id": "io-aaaaaaaaaaaa", "issue_snapshot": snapshot,
			}, nil)
			if err == nil {
				t.Fatal("malformed issue_snapshot was silently accepted")
			}
		})
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
