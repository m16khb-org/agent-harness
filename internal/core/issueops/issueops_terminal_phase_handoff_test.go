package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

// validStrandedHandoff builds a valid recovery_required + cleanup_only envelope
// (the #2581 shape): the worktree was never live, preserved as a cleanup-only
// tombstone. Its phase is advanced to done by the caller.
func validStrandedHandoff(repo, worker string) *model.IssueOpsExecutionHandoff {
	return &model.IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion,
		State:           handoff.StateRecoveryRequired,
		Attempt:         1,
		OwnershipEpoch:  "epoch-1",
		AttemptBaseHead: strings.Repeat("b", 40),
		Driver:          "orca",
		Agent:           "codex",
		CoordinatorRoot: repo,
		WorkerRoot:      worker,
		Orca: &model.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/1-demo", WorktreeInstanceID: "inst-1",
		},
		CleanupOnly: &model.IssueOpsOrcaCleanupArtifact{
			Kind: "worktree", ID: "wt-1", InstanceID: "inst-1", Path: worker, Reason: "nested worktree path did not match canonical IssueOps path",
		},
		Failure: &model.IssueOpsExecutionHandoffFailure{Code: "worktree_cleanup_only", Message: "path mismatch", At: "2026-07-15T00:00:00Z"},
	}
}

// TestTerminalPhaseHandoffGuardRejectsNonTerminalHandoff pins the Task F3
// write-time guard for every handoff state.
func TestTerminalPhaseHandoffGuardRejectsNonTerminalHandoff(t *testing.T) {
	base := IssueOpsRecord{ID: "io-guard", Repo: "/tmp/repo"}
	for _, state := range []string{handoff.StateCoordinatorPreparing, handoff.StateDispatched, handoff.StateClaimed, handoff.StateSubmitted, handoff.StateRecoveryRequired} {
		r := base
		r.ExecutionHandoff = &model.IssueOpsExecutionHandoff{State: state}
		err := issueOpsTerminalPhaseHandoffGuard(r, IssueOpsPhaseDone)
		if err == nil {
			t.Fatalf("done transition must be rejected while handoff state=%s", state)
		}
		if !strings.Contains(err.Error(), "handoff recover") || !strings.Contains(err.Error(), r.ID) {
			t.Fatalf("guard error must name the recover escape and id for state=%s: %v", state, err)
		}
	}
	// closed handoff, nil handoff, and non-terminal target phase are all allowed.
	for _, tc := range []struct {
		name  string
		rec   IssueOpsRecord
		phase IssueOpsPhase
	}{
		{"closed handoff", IssueOpsRecord{ID: "io-x", ExecutionHandoff: &model.IssueOpsExecutionHandoff{State: handoff.StateClosed}}, IssueOpsPhaseDone},
		{"no handoff", IssueOpsRecord{ID: "io-x"}, IssueOpsPhaseDone},
		{"non-terminal phase", IssueOpsRecord{ID: "io-x", ExecutionHandoff: &model.IssueOpsExecutionHandoff{State: handoff.StateRecoveryRequired}}, IssueOpsPhasePR},
	} {
		if err := issueOpsTerminalPhaseHandoffGuard(tc.rec, tc.phase); err != nil {
			t.Fatalf("%s must be allowed: %v", tc.name, err)
		}
	}
}

// TestForceDoneRejectsNonTerminalHandoff proves --force cannot strand the
// inconsistency: force-done from PR phase is rejected while the handoff is
// non-terminal, pointing at recover.
func TestForceDoneRejectsNonTerminalHandoff(t *testing.T) {
	stateRoot := t.TempDir()
	seed := IssueOpsRecord{ID: "io-force-strand", Phase: IssueOpsPhasePR, Repo: "/tmp/repo", Branch: "1-demo"}
	seed.ExecutionHandoff = validStrandedHandoff("/tmp/repo", "/tmp/repo.worktrees/1-demo")
	if _, err := WriteIssueOps(stateRoot, seed); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}
	_, err := ForceDoneIssueOps(stateRoot, seed.ID)
	if err == nil || !strings.Contains(err.Error(), "handoff recover") {
		t.Fatalf("force-done must reject a non-terminal handoff with a recover pointer: %v", err)
	}
	// The record must not have been advanced.
	after, readErr := ReadIssueOps(stateRoot, seed.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if after.Phase == IssueOpsPhaseDone {
		t.Fatalf("force-done must not have advanced the stranded record to done")
	}
}
