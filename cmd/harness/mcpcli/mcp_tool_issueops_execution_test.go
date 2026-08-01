package mcpcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
)

func TestMCPExecutionDependenciesPropagatePublicationReconcileWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueops.RemotePullRequestReconcileHandler(func(context.Context, string, issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
		invoked++
		return issueops.ExecutionReconcileResult{}, nil
	})

	deps := issueOpsExecutionActionDependencies(MCPDependencies{Publication: issueops.RemotePublicationHandlers{Reconcile: handler}})
	if deps.RemoteReconcile == nil {
		t.Fatal("publication reconcile handler was not propagated")
	}
	if reflect.ValueOf(deps.RemoteReconcile).Pointer() != reflect.ValueOf(handler).Pointer() {
		t.Fatal("publication reconcile handler changed during MCP dependency mapping")
	}
	if invoked != 0 {
		t.Fatalf("publication reconcile handler invoked during propagation: %d", invoked)
	}
}

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

func TestExecutionActionRequestFromMCPMapsResume(t *testing.T) {
	req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "resume", "id": "io-aaaaaaaaaaaa",
		"expected_generation": float64(3),
		"host":                "codex",
		"session_id":          "session-resume",
		"session_pid":         float64(42),
		"session_started_at":  "2026-07-30T00:00:00Z",
		"session_executable":  "/usr/local/bin/codex",
		"cwd":                 "/repo.worktrees/resume",
		"confirm":             true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Action != "resume" || req.ID != "io-aaaaaaaaaaaa" || req.ExpectedGeneration != 3 ||
		req.Actor.Host != "codex" || req.Actor.SessionID != "session-resume" ||
		req.Actor.SessionProcess == nil || req.Actor.SessionProcess.PID != 42 ||
		req.CWD != "/repo.worktrees/resume" || !req.Confirm || req.IssueSnapshot != nil {
		t.Fatalf("MCP resume request drifted: %#v", req)
	}
}

func TestHandleToolCallWithDependenciesRoutesResumeToInjectedHandler(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	params, err := json.Marshal(MCPToolCall{Name: "issueops_execution", Arguments: map[string]any{
		"action": "resume", "id": "io-aaaaaaaaaaaa", "expected_generation": float64(3),
		"host": "codex", "session_id": "session-resume", "session_pid": float64(42),
		"session_started_at": "2026-07-31T00:00:00Z", "session_executable": "/usr/local/bin/codex",
		"cwd": "/repo.worktrees/resume", "confirm": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	response, rpcErr := HandleToolCallWithDependencies(params, MCPDependencies{Resume: func(_ context.Context, stateRoot string, request issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
		calls++
		if stateRoot == "" || request.ID != "io-aaaaaaaaaaaa" || request.ExpectedGeneration != 3 || request.CWD != "/repo.worktrees/resume" || !request.Confirm {
			t.Fatalf("resume handler request=%+v state_root=%q", request, stateRoot)
		}
		return issueops.ExecutionResumeResult{OK: true, ID: request.ID}, nil
	}})
	if rpcErr != nil || calls != 1 {
		t.Fatalf("resume MCP rpc_err=%v calls=%d", rpcErr, calls)
	}
	payload, ok := response.(map[string]any)
	if !ok {
		t.Fatalf("resume MCP response type=%T", response)
	}
	content, ok := payload["content"].([]map[string]any)
	if !ok || len(content) != 1 || !strings.Contains(content[0]["text"].(string), `"id": "io-aaaaaaaaaaaa"`) {
		t.Fatalf("resume MCP response=%#v", response)
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
