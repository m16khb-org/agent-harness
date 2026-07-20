package lifecycle

import (
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestIssueOpsFenceResourceTargetsMatchCLIAndMCP(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, "claimed")
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-worker"
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-worker"
	record.ExecutionHandoff.Orca.TaskID = "task-worker"
	record.ExecutionHandoff.Orca.DispatchID = "dispatch-worker"
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}

	for _, request := range []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "Bash", Command: "orca terminal close --terminal term-worker --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "Bash", Command: "orca orchestration task-update --id task-worker --status failed --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "Bash", Command: "orca orchestration dispatch --task task-worker --dispatch-id dispatch-worker --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__orca__terminal_close", ToolInput: map[string]any{"terminal": "term-worker"}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__orca__orchestration_task_update", ToolInput: map[string]any{"id": "task-worker"}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__orca__orchestration_dispatch", ToolInput: map[string]any{"task_id": "task-worker", "dispatch_id": "dispatch-worker"}, EnforceWorktree: true, SourceCheckout: repo},
	} {
		got, ok, reason := selectSupervisedHandoffRecord(request)
		if !ok || reason != "" || got.ID != record.ID {
			t.Fatalf("persisted Orca resource must select its record: request=%#v got=%#v ok=%v reason=%q", request, got, ok, reason)
		}
	}

	for _, request := range []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "Bash", Command: "orca terminal close --terminal term-unrelated --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "Bash", Command: "orca orchestration task-update --id task-unrelated --status failed --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__orca__terminal_close", ToolInput: map[string]any{"terminal_handle": "term-unrelated"}, EnforceWorktree: true, SourceCheckout: repo},
	} {
		if _, ok, reason := selectSupervisedHandoffRecord(request); ok || reason != "" {
			t.Fatalf("literal unrelated Orca resource must remain source work: request=%#v ok=%v reason=%q", request, ok, reason)
		}
	}

	request := HookToolUseLifecycleRequest{Repo: repo, CWD: worktree, Tool: "Bash", Command: "orca terminal close --terminal $TERM --json", EnforceWorktree: true, SourceCheckout: repo}
	if _, ok, reason := selectSupervisedHandoffRecord(request); ok || reason == "" {
		t.Fatalf("dynamic worker-root resource control must fail closed: ok=%v reason=%q", ok, reason)
	}
}

func TestProtectedOrcaResourcesIncludeWorkspaceAndRejectDuplicateOwnership(t *testing.T) {
	workspace := IssueOpsRecord{ID: "io-workspace", ExecutionWorkspace: &issueopsmodel.IssueOpsExecutionWorkspace{Orca: &issueopsmodel.IssueOpsOrcaIdentity{WorktreeID: "wt-workspace"}}}
	targets, control, literal := protectedOrcaResourceTargets(HookToolUseLifecycleRequest{Tool: "Bash", Command: "orca worktree remove --id wt-workspace"})
	if !control || !literal || len(targets) != 1 || !recordOwnsProtectedOrcaResource(workspace, targets[0]) {
		t.Fatalf("workspace worktree must be protected: targets=%#v control=%v literal=%v", targets, control, literal)
	}
	duplicate := workspace
	duplicate.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{Orca: &issueopsmodel.IssueOpsOrcaIdentity{WorktreeID: "wt-workspace"}}
	request := HookToolUseLifecycleRequest{Repo: "/repo", CWD: "/repo", Tool: "Bash", Command: "orca worktree remove --id wt-workspace"}
	if _, reason := recordsMatchingProtectedOrcaResource(request, []IssueOpsRecord{duplicate}); reason == "" {
		t.Fatal("duplicate workspace/handoff worktree identity must fail closed")
	}
	if got := classifyHandoffFenceScope(request, []IssueOpsRecord{duplicate}); got != handoffFenceScopeAmbiguousCrossRoot {
		t.Fatalf("duplicate resource scope = %q", got)
	}
}
