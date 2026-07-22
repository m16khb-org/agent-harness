package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestWorkspaceScopeLeavesOrdinarySourceWorkUnfenced(t *testing.T) {
	record := workspaceAuthorityRecord(IssueOpsRecord{ID: "io-workspace", Repo: "/repo"}, &issueopsmodel.IssueOpsExecutionWorkspace{State: "ready", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature"})
	source := HookToolUseLifecycleRequest{Repo: "/repo", CWD: "/repo", Tool: "Bash", Command: "git status --short"}
	if got := classifyHandoffFenceScope(source, []IssueOpsRecord{record}); got != handoffFenceScopeSourceOnly {
		t.Fatalf("ordinary source work scope = %q", got)
	}
	worker := HookToolUseLifecycleRequest{Repo: "/repo.worktrees/feature", CWD: "/repo.worktrees/feature", Tool: "Bash", Command: "git status --short"}
	if got := classifyHandoffFenceScope(worker, []IssueOpsRecord{record}); got != handoffFenceScopeWorkerOrCycleTargeted {
		t.Fatalf("workspace-root work scope = %q", got)
	}
}

func TestWorkspaceScopeTreatsGoRecursivePackagePatternAsSourceOnly(t *testing.T) {
	source := t.TempDir()
	worker := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "feature")
	if err := os.MkdirAll(filepath.Join(source, "internal", "core", "lifecycle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worker, 0o755); err != nil {
		t.Fatal(err)
	}
	record := workspaceAuthorityRecord(IssueOpsRecord{ID: "io-workspace", Repo: source}, &issueopsmodel.IssueOpsExecutionWorkspace{State: "ready", CoordinatorRoot: source, WorkerRoot: worker})
	for _, command := range []string{
		"go test ./... -count=1",
		"go test ./internal/core/lifecycle -count=1",
		"go test ./internal/core/issueops -count=1",
		"go test ./internal/core/commandparse ./internal/core/issueops ./internal/core/lifecycle -count=1",
	} {
		req := HookToolUseLifecycleRequest{Repo: worker, CWD: source, SourceCheckout: source, Tool: "Bash", Command: command}
		if got := classifyHandoffFenceScope(req, []IssueOpsRecord{record}); got != handoffFenceScopeSourceOnly {
			t.Fatalf("ordinary source Go test scope = %q for %q", got, command)
		}
	}
}

func TestLiteralNewCycleStartUsesExactSourceCWDDespiteStaleRepoHint(t *testing.T) {
	source := t.TempDir()
	worker := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "active")
	record := workspaceAuthorityRecord(IssueOpsRecord{ID: "io-active", Repo: source}, &issueopsmodel.IssueOpsExecutionWorkspace{State: "ready", CoordinatorRoot: source, WorkerRoot: worker})
	req := HookToolUseLifecycleRequest{Repo: worker, CWD: source, SourceCheckout: source, Tool: "Bash", Command: "agent-harness issueops start --repo " + source + " --branch 52-next --json"}
	if got := classifyHandoffFenceScope(req, []IssueOpsRecord{record}); got != handoffFenceScopeSourceOnly {
		t.Fatalf("literal source cycle start scope = %q", got)
	}
}

func TestReadyWorkspaceRequiresSealedPreparationSessionAtIsolatedRoot(t *testing.T) {
	record := workspaceAuthorityRecord(IssueOpsRecord{ID: "io-workspace", Repo: "/repo"}, &issueopsmodel.IssueOpsExecutionWorkspace{State: "ready", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature", PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"}})
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

func TestRecoveryWorkspaceAllowsOnlyExactSourceReconciliation(t *testing.T) {
	record := workspaceAuthorityRecord(IssueOpsRecord{ID: "io-workspace", Repo: "/repo"}, &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "recovery_required", WorkspaceEpoch: "epoch-1", Agent: "codex", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature",
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"},
	})
	command := "agent-harness issueops worktree reconcile --id io-workspace --workspace-epoch epoch-1 --host codex --session-id session-1 --agent-id agent-1 --source-cwd /repo --json"
	req := HookToolUseLifecycleRequest{SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "session-1", AgentID: "agent-1", Tool: "Bash", Command: command}
	if !allowedExactHandoffLifecycleCommand(req, record) {
		t.Fatal("exact sealed workspace reconciliation was blocked")
	}
	for _, invalid := range []string{
		strings.Replace(command, "epoch-1", "epoch-other", 1),
		strings.Replace(command, "session-1", "session-other", 1),
		strings.Replace(command, "--source-cwd /repo", "--source-cwd /other", 1),
	} {
		req.Command = invalid
		if allowedExactHandoffLifecycleCommand(req, record) {
			t.Fatalf("inexact reconciliation was allowed: %s", invalid)
		}
	}
}

func TestReadyWorkspacePreparationCommandsRequireSealedWorkerSession(t *testing.T) {
	record := workspaceAuthorityRecord(IssueOpsRecord{ID: "io-workspace", Repo: "/repo"}, &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: "epoch-1", Agent: "codex", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature",
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"},
	})
	command := "agent-harness issueops worktree prepare-tools --id io-workspace --host codex --session-id session-1 --agent-id agent-1 --cwd /repo.worktrees/feature --json"
	worker := HookToolUseLifecycleRequest{SourceCheckout: "/repo", CWD: "/repo.worktrees/feature", Host: "codex", SessionID: "session-1", AgentID: "agent-1", Tool: "Bash", Command: command}
	if !allowedExactHandoffLifecycleCommand(worker, record) {
		t.Fatal("sealed preparer at the worker root was blocked")
	}
	source := worker
	source.CWD = "/repo"
	if !allowedExactHandoffLifecycleCommand(source, record) {
		t.Fatal("sealed source preparer with an exact worker cwd flag was blocked")
	}
	foreign := worker
	foreign.CWD = "/other"
	if allowedExactHandoffLifecycleCommand(foreign, record) {
		t.Fatal("unrelated root was allowed to run worker preparation")
	}
}

func TestReadyWorkspaceStatusAllowsExactCycleObservationFromSource(t *testing.T) {
	record := workspaceAuthorityRecord(IssueOpsRecord{ID: "io-workspace", Repo: "/repo"}, &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature",
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"},
	})
	req := HookToolUseLifecycleRequest{
		SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "session-1", AgentID: "agent-1",
		Tool: "Bash", Command: "agent-harness issueops status --id io-workspace --json",
	}
	if !allowedExactHandoffLifecycleCommand(req, record) {
		t.Fatal("exact ready-workspace status observation from source was blocked")
	}
}

func TestReadyWorkspaceSourcePreparationMayChangeOnlyLinkedPlan(t *testing.T) {
	record := workspaceAuthorityRecord(IssueOpsRecord{
		ID: "io-workspace", Repo: "/repo", PlanPath: "/repo.worktrees/feature/.agent-harness/plans/feature.md",
	}, &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/feature",
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"},
	})
	base := HookToolUseLifecycleRequest{SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "session-1", AgentID: "agent-1"}

	edit := base
	edit.Tool = "apply_patch"
	edit.Paths = []string{record.PlanPath, record.PlanPath}
	if !allowedSourceWorkspacePlanMutation(edit, record) {
		t.Fatal("sealed source preparer could not edit the exact linked plan")
	}
	edit.SourceCheckout = ""
	if !allowedSourceWorkspacePlanMutation(edit, record) {
		t.Fatal("sealed source preparer with an omitted source-checkout hint was blocked")
	}

	commit := base
	commit.Tool = "Bash"
	commit.Command = "git -C /repo.worktrees/feature commit --only .agent-harness/plans/feature.md -m 'docs(issueops): revise handoff plan'"
	if !allowedSourceWorkspacePlanMutation(commit, record) {
		t.Fatal("sealed source preparer could not commit only the exact linked plan")
	}

	for _, denied := range []HookToolUseLifecycleRequest{
		{SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "session-1", AgentID: "agent-1", Tool: "apply_patch", Paths: []string{"/repo.worktrees/feature/internal/x.go"}},
		{SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "other", AgentID: "agent-1", Tool: "apply_patch", Paths: []string{record.PlanPath}},
		{SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "session-1", AgentID: "agent-1", Tool: "Bash", Command: "git -C /repo.worktrees/feature commit -am 'too broad'"},
		{SourceCheckout: "/repo", CWD: "/repo", Host: "codex", SessionID: "session-1", AgentID: "agent-1", Tool: "Bash", Command: "git -C /repo.worktrees/feature commit --only internal/x.go -m 'wrong file'"},
	} {
		if allowedSourceWorkspacePlanMutation(denied, record) {
			t.Fatalf("source preparation escaped the exact linked-plan boundary: %#v", denied)
		}
	}
}

func workspaceAuthorityRecord(record IssueOpsRecord, workspace *issueopsmodel.IssueOpsExecutionWorkspace) IssueOpsRecord {
	record.CycleState = issueopsmodel.IssueOpsCycleActive
	record.Ownership = &issueopsmodel.IssueOpsOwnershipLedger{
		ActiveAttempt: 1,
		Attempts:      []issueopsmodel.IssueOpsOwnershipAttempt{{Number: 1, Workspace: workspace}},
	}
	return record
}
