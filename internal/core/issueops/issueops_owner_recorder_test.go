package issueops

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
)

func TestOwnerRequiredForPostHandoffRecorders(t *testing.T) {
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)

	if _, err := AdvanceIssueOpsPhaseWithActor(stateRoot, record.ID, string(record.Phase), owner); err != nil {
		t.Fatalf("exact owner phase recorder: %v", err)
	}
	if _, err := RecordIssueOpsAISlopCleanEvidenceWithActor(stateRoot, record.ID, []string{"minimal-diff"}, []string{"go test ./..."}, owner); err != nil {
		t.Fatalf("exact owner evidence recorder: %v", err)
	}
	if _, err := AddIssueOpsFeedbackWithActor(stateRoot, record.ID, "review", "update contract", "contract_change", owner); err != nil {
		t.Fatalf("exact owner feedback add: %v", err)
	}
	if _, err := ResolveIssueOpsFeedbackWithActor(stateRoot, record.ID, 0, "valid-defect", owner); err != nil {
		t.Fatalf("exact owner feedback resolve: %v", err)
	}
	if _, err := MarkIssueOpsContractFeedbackIssueUpdatedWithActor(stateRoot, record.ID, owner); err != nil {
		t.Fatalf("exact owner issue-update marker: %v", err)
	}

	for _, actor := range []IssueOpsActor{
		{Host: owner.Host, SessionID: "source-session", AgentID: owner.AgentID, CWD: record.Repo},
		{Host: owner.Host, SessionID: "other-session", AgentID: owner.AgentID, CWD: owner.CWD},
		{},
	} {
		if _, err := AdvanceIssueOpsPhaseWithActor(stateRoot, record.ID, string(record.Phase), actor); err == nil || !strings.Contains(err.Error(), "ownership transfer") {
			t.Fatalf("non-owner phase recorder must fail closed: actor=%#v err=%v", actor, err)
		}
	}
}

func ownershipActiveRecorderRecord(t *testing.T) (string, IssueOpsRecord, IssueOpsActor) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "16-owner"
	worker := issueOpsWorktreePathForTest(repo, branch)
	if code, _, stderr := preflight.GitCmd(repo, "branch", branch, "main"); code != 0 {
		t.Fatalf("git branch: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worker, branch); code != 0 {
		t.Fatalf("git worktree add: %s", stderr)
	}
	baseHead := strings.TrimSpace(preflight.GitOut(worker, "rev-parse", "HEAD"))
	writeIssueOpsFile(t, worker, "plans/demo.md", "# ownership plan\n")
	for _, args := range [][]string{{"add", "plans/demo.md"}, {"commit", "-q", "-m", "docs: add ownership plan"}} {
		if code, _, stderr := preflight.GitCmd(worker, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	head := strings.TrimSpace(preflight.GitOut(worker, "rev-parse", "HEAD"))
	owner := &model.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "owner-session", AgentID: "owner-agent"}
	preparation := &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "source-session", AgentID: "source-agent"}
	orca := &model.IssueOpsOrcaIdentity{RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/" + branch, WorktreeID: "wt-1", WorktreeInstanceID: "instance-1", WorktreePath: worker, WorkerPTYID: "pty-1", WorkerTerminalHandle: "term-1", WorkerMailboxHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1"}
	workspace := &model.IssueOpsExecutionWorkspace{State: "ready", WorkspaceEpoch: "workspace-1", Driver: "orca", Agent: "claude", CoordinatorRoot: repo, WorkerRoot: worker, PreparationSession: preparation, BaseHead: baseHead}
	h := &model.IssueOpsExecutionHandoff{
		State: handoff.StateOwnerActive, Attempt: 1, OwnershipEpoch: "ownership-1", WorkspaceEpoch: "workspace-1", WorkspaceSHA256: strings.Repeat("a", 64),
		AttemptBaseHead: head, ContextVersion: handoff.ContextVersion,
		Driver: "orca", Agent: "claude", DeliveryMode: "inject", CoordinatorRoot: repo, CoordinatorMailboxHandle: "term-source", WorkerRoot: worker, OwnerSession: owner,
		Orientation: &model.IssueOpsOwnershipOrientation{IssueURL: "https://github.com/acme/repo/issues/16", PlanSHA256: strings.Repeat("d", 64), Understanding: "sealed cycle only", ScopeConfirmation: "worker root only", RecordedAt: "2026-07-20T00:00:00Z"}, Orca: orca,
	}
	record := IssueOpsRecord{
		SchemaVersion: IssueOpsCurrentSchemaVersion, OK: true, ID: NewIssueOpsID(repo, branch), Repo: repo, Branch: branch,
		IssueURL: "https://github.com/acme/repo/issues/16", Phase: IssueOpsPhaseCompatibilityReview, PlanPath: filepath.Join(worker, "plans", "demo.md"), WorktreePath: worker,
		BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/acme/repo/issues/16", Branch: branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true},
		CycleState:    IssueOpsCycleActive,
		Ownership:     &IssueOpsOwnershipLedger{ActiveAttempt: 1, Attempts: []IssueOpsOwnershipAttempt{{Number: 1, Workspace: workspace, Handoff: h, StartedAt: "2026-07-20T00:00:00Z"}}},
		CreatedAt:     "2026-07-20T00:00:00Z", UpdatedAt: "2026-07-20T00:00:00Z",
	}
	packet, err := handoff.BuildContext(record, handoff.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h.ContextSHA256 = packet.SHA256
	h.ContextSourceSHA256 = packet.SourceSHA256
	active, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, active, IssueOpsActor{Host: owner.Host, SessionID: owner.SessionID, AgentID: owner.AgentID, CWD: worker}
}
