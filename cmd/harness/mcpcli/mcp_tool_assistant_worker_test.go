package mcpcli

import (
	workercontract "agent-harness/internal/contract/worker"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleAssistantWorkerMCPToolCallCoversLocalAssistantPayloads(t *testing.T) {
	repo := makeGitRepoForContract(t)
	t.Setenv("HARNESS_DAEMON_DIR", t.TempDir())
	tests := []struct {
		name     string
		call     MCPToolCall
		wantText string
	}{
		{name: "daemon status", call: MCPToolCall{Name: "daemon_status", Arguments: map[string]any{}}, wantText: "daemon is not running"},
		{name: "contract schema", call: MCPToolCall{Name: "contract_schema", Arguments: map[string]any{}}, wantText: "mcp_tools"},
		{name: "contract check", call: MCPToolCall{Name: "contract_check", Arguments: map[string]any{}}, wantText: "mcp_tools"},
		{name: "commit suggest no diff", call: MCPToolCall{Name: "commit_suggest", Arguments: map[string]any{"repo": repo}}, wantText: `"executed": false`},
		{name: "lint diagnose success", call: MCPToolCall{Name: "lint_diagnose", Arguments: map[string]any{"repo": repo, "command_argv": []any{"git", "status", "--short"}}}, wantText: `"failed": false`},
		{name: "web fetch safety rejection", call: MCPToolCall{Name: "web_fetch_resilient", Arguments: map[string]any{"url": "http://127.0.0.1/private"}}, wantText: `"stop_reason": "safety_rejected"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleAssistantWorkerMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err != nil {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
			if text := mcpAssistantWorkerPayloadText(t, outcome.Payload); !strings.Contains(text, tc.wantText) {
				t.Fatalf("payload text = %s, want %q", text, tc.wantText)
			}
		})
	}
}

func TestHandleAssistantWorkerMCPToolCallCoversWorkerLifecyclePayloads(t *testing.T) {
	t.Setenv("HARNESS_WORKER_DIR", t.TempDir())
	enqueue := handleAssistantWorkerMCPToolCall(MCPToolCall{Name: "worker_enqueue", Arguments: map[string]any{
		"kind": "qa", "payload": "check docs",
	}})
	if !enqueue.Handled || enqueue.Err != nil {
		t.Fatalf("unexpected worker_enqueue outcome: %#v", enqueue)
	}
	var job workercontract.WorkerJob
	decodeAssistantWorkerPayload(t, enqueue.Payload, &job)
	if job.Status != workercontract.WorkerStatusQueued || job.Kind != "qa" {
		t.Fatalf("unexpected enqueued job: %+v", job)
	}

	for _, tc := range []struct {
		name     string
		call     MCPToolCall
		wantText string
	}{
		{name: "worker status", call: MCPToolCall{Name: "worker_status", Arguments: map[string]any{"id": job.ID}}, wantText: job.ID},
		{name: "worker list", call: MCPToolCall{Name: "worker_list", Arguments: map[string]any{}}, wantText: job.ID},
		{name: "worker cancel", call: MCPToolCall{Name: "worker_cancel", Arguments: map[string]any{"id": job.ID}}, wantText: workercontract.WorkerStatusCancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleAssistantWorkerMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err != nil {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
			if text := mcpAssistantWorkerPayloadText(t, outcome.Payload); !strings.Contains(text, tc.wantText) {
				t.Fatalf("payload text = %s, want %q", text, tc.wantText)
			}
		})
	}
}

func TestHandleAssistantWorkerMCPToolCallCoversWorkerErrorsAndUnknownTool(t *testing.T) {
	t.Setenv("HARNESS_WORKER_DIR", t.TempDir())
	enqueue := handleAssistantWorkerMCPToolCall(MCPToolCall{Name: "worker_enqueue", Arguments: map[string]any{"kind": ""}})
	if !enqueue.Handled || enqueue.Err == nil || enqueue.Err.Code != -32000 || enqueue.Err.Message != "worker_enqueue failed" {
		t.Fatalf("unexpected worker_enqueue error outcome: %#v", enqueue)
	}

	status := handleAssistantWorkerMCPToolCall(MCPToolCall{Name: "worker_status", Arguments: map[string]any{"id": "../bad"}})
	if !status.Handled || status.Err == nil || status.Err.Code != -32000 || status.Err.Message != "worker_status failed" {
		t.Fatalf("unexpected worker_status error outcome: %#v", status)
	}

	unknown := handleAssistantWorkerMCPToolCall(MCPToolCall{Name: "not_assistant_worker", Arguments: map[string]any{}})
	if unknown.Handled {
		t.Fatalf("unknown assistant/worker tool should pass through: %#v", unknown)
	}
}

func mcpAssistantWorkerPayloadText(t *testing.T, payload any) string {
	t.Helper()
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func decodeAssistantWorkerPayload(t *testing.T, payload any, target any) {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		t.Fatal(err)
	}
}
