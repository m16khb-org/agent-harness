package lifecycle

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestHandoffGuardBlocksBeforeClaim(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "block" || !strings.Contains(got.Reason, "claim") {
		t.Fatalf("pre-claim mutation should block: %#v", got)
	}
}

func TestHandoffGuardAllowsMatchingClaimedWorkerInTree(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "allow" {
		t.Fatalf("claimed worker in tree should pass: %#v", got)
	}
}

func TestHandoffGuardBlocksCoordinatorAbsolutePathIntoWorkerTree(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, repo, "codex", "coordinator", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "block" {
		t.Fatalf("coordinator absolute-path edit should block: %#v", got)
	}
}

func TestHandoffGuardBlocksWrongOrRestartedSession(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, session := range []string{"other", ""} {
		got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", session, filepath.Join(worktree, "internal", "x.go")))
		if got.Decision != "block" {
			t.Fatalf("session %q should block: %#v", session, got)
		}
	}
}

func TestHandoffGuardBlocksWorkerEscape(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(repo, "internal", "x.go")))
	if got.Decision != "block" || !strings.Contains(got.Reason, "outside") {
		t.Fatalf("worker escape should block: %#v", got)
	}
}

func TestHandoffGuardAllowsExactLifecycleCommandsOnly(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	claim := handoffEditRequest(record, worktree, "codex", "session-1", "")
	claim.Tool = "Bash"
	claim.Command = "agent-harness issueops handoff claim --id " + record.ID + " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64)
	if got := BuildLifecyclePreToolUseDecision(claim); got.Decision != "allow" {
		t.Fatalf("exact claim command should pass: %#v", got)
	}
	claim.Command += " && touch internal/x.go"
	if got := BuildLifecyclePreToolUseDecision(claim); got.Decision != "block" {
		t.Fatalf("compound lifecycle command should block: %#v", got)
	}
}

func TestSessionStartRendersClaimWithoutMutation(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	before, _ := json.Marshal(record)
	guidance := BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "worker-1")
	for _, want := range []string{"role=worker", "handoff claim", "--id " + record.ID, "--attempt 1", "--ownership-epoch epoch-1", "--context-sha256 " + strings.Repeat("a", 64), "--session-id session-1"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
	afterRecord, err := ReadIssueOps(IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(afterRecord)
	if string(before) != string(after) {
		t.Fatal("SessionStart guidance mutated IssueOps state")
	}
}

func lifecycleHandoffRecord(t *testing.T, state string) (string, IssueOpsRecord, string) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-demo", IssueOpsPhaseImplement)
	record, ok := ActiveIssueOpsCycleForBranch(repo, "1-demo")
	if !ok {
		t.Fatal("active record missing")
	}
	worktree := makeIssueOpsGuardWorktreeForTest(t, repo, "1-demo")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-demo")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion, State: state, Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64),
		CoordinatorRoot: repo, WorkerRoot: worktree, Orca: &issueopsmodel.IssueOpsOrcaIdentity{WorktreeID: "wt-1", WorktreePath: worktree},
	}
	if state == handoff.StateClaimed {
		record.ExecutionHandoff.WorkerSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "worker-1"}
	}
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return repo, record, worktree
}

func handoffEditRequest(record IssueOpsRecord, cwd, host, session, target string) HookToolUseLifecycleRequest {
	paths := []string(nil)
	if target != "" {
		paths = []string{target}
	}
	return HookToolUseLifecycleRequest{
		Repo: cwd, CWD: cwd, Host: host, SessionID: session, AgentID: "worker-1", Tool: "Edit", Paths: paths,
		EnforceWorktree: true, ExpectedWorktree: record.WorktreePath, SourceCheckout: record.Repo,
	}
}
