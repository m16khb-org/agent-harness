package issueops

import (
	"context"
	"testing"

	"agent-harness/internal/port"
)

func TestCancellationQuiescenceAllowsFailedTaskWithStaleExactDispatch(t *testing.T) {
	record, client := soleWriterOwnedDispatchFixture()
	client.terminals = nil
	client.tasks = nil
	client.allTasks = []port.OrcaTask{{ID: "task_owner", Status: "failed"}}

	if err := requireCancellationQuiescence(context.Background(), record, client, "2026-07-22T00:00:00Z"); err != nil {
		t.Fatalf("failed task with exact stale dispatch did not quiesce: %v", err)
	}
	client.allTasks[0].Status = "completed"
	if err := requireCancellationQuiescence(context.Background(), record, client, "2026-07-22T00:00:00Z"); err == nil {
		t.Fatal("non-failed task with a stale dispatch quiesced")
	}
}
