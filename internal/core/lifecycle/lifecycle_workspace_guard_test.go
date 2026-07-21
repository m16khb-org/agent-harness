package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
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

func TestReadyWorkspaceAllowsOnlySourceCoordinatorPlanCheckpoint(t *testing.T) {
	repo, record, worker := lifecycleHandoffRecord(t, "coordinator_preparing")
	preparer := &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "coordinator", AgentID: "worker-1"}
	record.ExecutionWorkspace = &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: "workspace-epoch-1", Driver: "orca", Agent: "codex",
		CoordinatorRoot: repo, WorkerRoot: worker, PreparationSession: preparer,
		BaseHead: strings.Repeat("b", 40),
	}
	record.ExecutionHandoff = nil
	var err error
	if record, err = writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	plan := filepath.Join(worker, ".agent-harness", "plans", record.ID+"-checkpoint.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	edit := handoffEditRequest(record, repo, "codex", "coordinator", plan)
	if got := BuildLifecyclePreToolUseDecision(edit); got.Decision != "allow" {
		t.Fatalf("sealed source coordinator plan edit blocked: %#v", got)
	}

	for _, command := range []string{
		"git -C " + worker + " add -- " + plan,
		"git -C " + worker + " commit --only -m 'docs: record current cycle plan' -- " + plan,
	} {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "Bash", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("sealed source coordinator plan command %q blocked: %#v", command, got)
		}
	}

	status := handoffEditRequest(record, repo, "codex", "coordinator", "")
	status.Tool = "Bash"
	status.Command = "agent-harness issueops status --id " + record.ID + " --json"
	if got := BuildLifecyclePreToolUseDecision(status); got.Decision != "allow" {
		t.Fatalf("ready workspace status observation blocked: %#v", got)
	}

	linkPlan := handoffEditRequest(record, repo, "codex", "coordinator", "")
	linkPlan.Tool = "Bash"
	linkPlan.Command = "agent-harness issueops link-plan --id " + record.ID + " --plan-path " + plan +
		" --host codex --session-id coordinator --agent-id worker-1 --cwd " + repo + " --json"
	if got := BuildLifecyclePreToolUseDecision(linkPlan); got.Decision != "allow" {
		t.Fatalf("exact sealed source coordinator link-plan blocked: %#v", got)
	}
	missingActor := linkPlan
	missingActor.Command = "agent-harness issueops link-plan --id " + record.ID + " --plan-path " + plan + " --json"
	if got := BuildLifecyclePreToolUseDecision(missingActor); got.Decision != "block" {
		t.Fatalf("actor-less link-plan gained preparation authority: %#v", got)
	}
	wrongActor := linkPlan
	wrongActor.Command = strings.Replace(linkPlan.Command, "--session-id coordinator", "--session-id other", 1)
	if got := BuildLifecyclePreToolUseDecision(wrongActor); got.Decision != "block" {
		t.Fatalf("different actor gained link-plan authority: %#v", got)
	}

	wrongSession := handoffEditRequest(record, repo, "codex", "other", plan)
	if got := BuildLifecyclePreToolUseDecision(wrongSession); got.Decision != "block" {
		t.Fatalf("different session gained plan checkpoint authority: %#v", got)
	}
	codeEdit := handoffEditRequest(record, repo, "codex", "coordinator", filepath.Join(worker, "internal", "x.go"))
	if got := BuildLifecyclePreToolUseDecision(codeEdit); got.Decision != "block" {
		t.Fatalf("source coordinator gained implementation authority: %#v", got)
	}
}
