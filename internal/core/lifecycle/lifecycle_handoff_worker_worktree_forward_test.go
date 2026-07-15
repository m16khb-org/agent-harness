package lifecycle

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

// TestWorkerWorktreeReadOnlyStaysAllowedAtCoordinatorPreparing pins the Task G3
// invariant that read-only IssueOps and git observation from inside the worker
// worktree remain allowed before dispatch (the allow set is unchanged).
func TestWorkerWorktreeReadOnlyStaysAllowedAtCoordinatorPreparing(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	readOnly := []string{
		"git status --short", "git diff --stat", "git log -1",
		"agent-harness issueops status --id " + record.ID + " --json",
		"agent-harness issueops resume --repo " + record.Repo + " --id " + record.ID + " --json",
	}
	for _, command := range readOnly {
		req := handoffEditRequest(record, worktree, "codex", "unclaimed-worker", "")
		req.Tool, req.Command = "Bash", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("worker-root read-only command must stay allowed pre-dispatch: command=%q got=%#v", command, got)
		}
	}
}

// TestWorkerWorktreeMutationBlockNamesCoordinatorDispatch proves G3: the
// pre-dispatch worker-worktree mutation block is no longer a silent dead-end —
// it names the source checkout and the handoff start forward action, while the
// deny decision itself is byte-identical to today (mutation still blocks).
func TestWorkerWorktreeMutationBlockNamesCoordinatorDispatch(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	req := handoffEditRequest(record, worktree, "codex", "unclaimed-worker", filepath.Join(worktree, "internal", "x.go"))
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" {
		t.Fatalf("mutation before claim must still block: %#v", got)
	}
	for _, want := range []string{"not dispatched", "handoff start", "source checkout", record.Repo, "read-only"} {
		if !strings.Contains(got.Reason, want) {
			t.Fatalf("worker-worktree block message missing forward pointer %q: %s", want, got.Reason)
		}
	}
	if strings.Contains(got.Reason, "remain read-only and poll") {
		t.Fatalf("G3 block message still uses the bare dead-end poll text: %s", got.Reason)
	}
}
