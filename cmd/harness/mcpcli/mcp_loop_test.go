package mcpcli

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/adapter/looprun"
)

func TestMCPLoopLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	start := handleLoopMCPToolCall(MCPToolCall{Name: "loop_start", Arguments: map[string]any{
		"repo": repo, "name": "mcp loop", "goal": "tests pass", "max_attempts": 2, "verify_argv": []any{"go", "test", "./..."},
	}})
	loop, ok := start.Payload.(looprun.LoopRun)
	if !ok || !loop.OK || loop.Status != "active" || len(loop.VerifyArgv) != 3 {
		t.Fatalf("unexpected loop start payload: %#v", start.Payload)
	}

	recordFail := handleLoopMCPToolCall(MCPToolCall{Name: "loop_record_attempt", Arguments: map[string]any{
		"id": loop.ID, "verdict": "fail", "evidence": []any{"one failure"},
	}})
	if loop, ok = recordFail.Payload.(looprun.LoopRun); !ok || len(loop.Attempts) != 1 || loop.Attempts[0].Verdict != "fail" {
		t.Fatalf("unexpected fail payload: %#v", recordFail.Payload)
	}

	recordPass := handleLoopMCPToolCall(MCPToolCall{Name: "loop_record_attempt", Arguments: map[string]any{
		"id": loop.ID, "verdict": "pass", "evidence": []any{"all green"},
	}})
	if loop, ok = recordPass.Payload.(looprun.LoopRun); !ok || len(loop.Attempts) != 2 || loop.Attempts[1].Verdict != "pass" {
		t.Fatalf("unexpected pass payload: %#v", recordPass.Payload)
	}

	stop := handleLoopMCPToolCall(MCPToolCall{Name: "loop_stop", Arguments: map[string]any{
		"id": loop.ID, "success": true,
	}})
	if loop, ok = stop.Payload.(looprun.LoopRun); !ok || loop.Status != "succeeded" {
		t.Fatalf("unexpected stop payload: %#v", stop.Payload)
	}

	statusOutcome := handleLoopMCPToolCall(MCPToolCall{Name: "loop_status", Arguments: map[string]any{"id": loop.ID}})
	status, ok := statusOutcome.Payload.(looprun.StatusResult)
	if !ok || !status.OK || status.AttemptCount != 2 || status.LastVerdict != "pass" {
		t.Fatalf("unexpected status payload: %#v", statusOutcome.Payload)
	}
}
