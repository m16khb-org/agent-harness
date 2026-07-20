package lifecycle

import (
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestWorkspaceScopeLeavesOrdinarySourceWorkUnfenced(t *testing.T) {
	record := IssueOpsRecord{ID: "io-workspace", Repo: "/repo", ExecutionWorkspace: &issueopsmodel.IssueOpsExecutionWorkspace{State: "ready", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature"}}
	source := HookToolUseLifecycleRequest{Repo: "/repo", CWD: "/repo", Tool: "Bash", Command: "git status --short"}
	if got := classifyHandoffFenceScope(source, []IssueOpsRecord{record}); got != handoffFenceScopeSourceOnly {
		t.Fatalf("ordinary source work scope = %q", got)
	}
	worker := HookToolUseLifecycleRequest{Repo: "/repo.worktrees/feature", CWD: "/repo.worktrees/feature", Tool: "Bash", Command: "git status --short"}
	if got := classifyHandoffFenceScope(worker, []IssueOpsRecord{record}); got != handoffFenceScopeWorkerOrCycleTargeted {
		t.Fatalf("workspace-root work scope = %q", got)
	}
}

func TestReadyWorkspaceRequiresSealedPreparationSessionAtIsolatedRoot(t *testing.T) {
	record := IssueOpsRecord{ID: "io-workspace", Repo: "/repo", ExecutionWorkspace: &issueopsmodel.IssueOpsExecutionWorkspace{State: "ready", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature", PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"}}}
	allowed := HookToolUseLifecycleRequest{SourceCheckout: "/repo", CWD: "/repo.worktrees/feature", Host: "codex", SessionID: "session-1", AgentID: "agent-1"}
	if reason := workspacePreparationBlockReason(allowed, record); reason != "" {
		t.Fatalf("sealed preparer blocked: %s", reason)
	}
	for _, request := range []HookToolUseLifecycleRequest{
		{SourceCheckout: "/repo", CWD: "/repo.worktrees/feature", Host: "codex", SessionID: "other", AgentID: "agent-1"},
		{SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "session-1", AgentID: "agent-1"},
		{SourceCheckout: "/other", CWD: "/repo.worktrees/feature", Host: "codex", SessionID: "session-1", AgentID: "agent-1"},
	} {
		if reason := workspacePreparationBlockReason(request, record); reason == "" {
			t.Fatalf("unsealed preparation was allowed: %#v", request)
		}
	}
}
