package mcpcli

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/workpool"
)

func TestMCPWorkpoolLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	create := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_create", Arguments: map[string]any{
		"repo": repo, "name": "mcp lifecycle", "size": 2, "lease_ttl": "1h",
	}})
	pool, ok := create.Payload.(workpool.WorkPool)
	if !ok || !pool.OK || pool.ID == "" || pool.Status != "active" {
		t.Fatalf("unexpected create payload: %#v", create.Payload)
	}

	for _, title := range []string{"first task", "second task", "third task"} {
		add := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_add_task", Arguments: map[string]any{
			"pool": pool.ID, "title": title, "instructions": "touch one scoped file", "scope": []any{"one file"}, "acceptance": []any{"evidence is recorded"},
		}})
		task, ok := add.Payload.(workpool.WorkTask)
		if !ok || !task.OK || task.PoolID != pool.ID || task.Status != "pending" {
			t.Fatalf("unexpected add payload: %#v", add.Payload)
		}
	}

	claimOutcome := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_claim", Arguments: map[string]any{
		"pool": pool.ID, "worker": "worker-a",
	}})
	claim, ok := claimOutcome.Payload.(workpool.ClaimResult)
	if !ok || !claim.OK || claim.Task.Status != "leased" || claim.Prompt == "" {
		t.Fatalf("unexpected claim payload: %#v", claimOutcome.Payload)
	}

	heartbeat := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_heartbeat", Arguments: map[string]any{
		"pool": pool.ID, "task": claim.Task.ID, "worker": "worker-a",
	}})
	if task, ok := heartbeat.Payload.(workpool.WorkTask); !ok || task.Status != "leased" {
		t.Fatalf("unexpected heartbeat payload: %#v", heartbeat.Payload)
	}

	submitted := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_submit", Arguments: map[string]any{
		"pool": pool.ID, "task": claim.Task.ID, "worker": "worker-a", "evidence": []any{"go test ./..."}, "branch": "workpool/first", "worktree": filepath.Join(repo, ".worktrees", "first"),
	}})
	if task, ok := submitted.Payload.(workpool.WorkTask); !ok || task.Status != "submitted" {
		t.Fatalf("unexpected submit payload: %#v", submitted.Payload)
	}

	accepted := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_accept", Arguments: map[string]any{
		"pool": pool.ID, "task": claim.Task.ID, "evidence": []any{"reviewed submitted evidence"},
	}})
	if task, ok := accepted.Payload.(workpool.WorkTask); !ok || task.Status != "accepted" {
		t.Fatalf("unexpected accept payload: %#v", accepted.Payload)
	}

	statusOutcome := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_status", Arguments: map[string]any{"pool": pool.ID}})
	status, ok := statusOutcome.Payload.(workpool.StatusResult)
	if !ok || !status.OK || status.Pool.ID != pool.ID || len(status.Tasks) != 3 || status.Counts["accepted"] != 1 || status.Counts["pending"] != 2 {
		t.Fatalf("unexpected status payload: %#v", statusOutcome.Payload)
	}

	closed := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_close", Arguments: map[string]any{
		"pool": pool.ID, "force": true, "reason": "test force closes remaining pending work",
	}})
	if pool, ok := closed.Payload.(workpool.WorkPool); !ok || pool.Status != "closed" {
		t.Fatalf("unexpected close payload: %#v", closed.Payload)
	}

	reap := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_reap", Arguments: map[string]any{"pool": pool.ID}})
	if tasks, ok := reap.Payload.([]workpool.WorkTask); !ok || len(tasks) != 0 {
		t.Fatalf("unexpected reap payload: %#v", reap.Payload)
	}
}

func TestMCPWorkpoolPilotArguments(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	create := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_create", Arguments: map[string]any{
		"repo": repo, "name": "mcp pilot", "pilot_required": true,
	}})
	pool, ok := create.Payload.(workpool.WorkPool)
	if !ok || !pool.PilotRequired {
		t.Fatalf("unexpected pilot create payload: %#v", create.Payload)
	}

	add := handleWorkpoolMCPToolCall(MCPToolCall{Name: "workpool_add_task", Arguments: map[string]any{
		"pool": pool.ID, "title": "pilot task", "instructions": "prove first", "pilot": true,
	}})
	pilot, ok := add.Payload.(workpool.WorkTask)
	if !ok || !pilot.OK {
		t.Fatalf("unexpected pilot add payload: %#v", add.Payload)
	}
	readPool, err := workpool.ReadPool(pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if readPool.PilotTaskID != pilot.ID {
		t.Fatalf("MCP pilot task id = %q, want %q", readPool.PilotTaskID, pilot.ID)
	}
}
