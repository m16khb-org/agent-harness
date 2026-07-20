package lifecycle

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

// strandedRecoveryRequiredRecord reproduces the #2581 shape: a supervised
// handoff preserved as recovery_required with a cleanup_only tombstone, whose
// phase was advanced to done, and whose worker worktree was never live. It is
// the only record fencing the source checkout.
func strandedRecoveryRequiredRecord(t *testing.T) (string, IssueOpsRecord, string) {
	t.Helper()
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	record.Phase = issueopsmodel.IssueOpsPhaseDone
	record.ExecutionHandoff.State = handoff.StateRecoveryRequired
	record.ExecutionHandoff.CleanupOnly = &issueopsmodel.IssueOpsOrcaCleanupArtifact{
		Kind: "worktree", ID: "wt-1", InstanceID: "inst-1", Path: worktree,
		Reason: "nested worktree path did not match canonical IssueOps path",
	}
	record.ExecutionHandoff.Failure = &issueopsmodel.IssueOpsExecutionHandoffFailure{
		Code: "worktree_cleanup_only", Message: "provisioning path mismatch", At: "2026-07-15T00:00:00Z",
	}
	// The worktree was never live; its Orca worktree handle must be cleared for a
	// cleanup-only tombstone (the artifact ID carries the id to release).
	if record.ExecutionHandoff.Orca != nil {
		record.ExecutionHandoff.Orca.WorktreeID = ""
		record.ExecutionHandoff.Orca.WorktreePath = ""
	}
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return repo, record, worktree
}

// TestStrandedHandoffBlockNamesRecoverEscape is the #2581 regression: a
// source-checkout mutation blocked by the stranded record must name the working
// `handoff recover` escape for its sub-state, never the misleading
// "flags do not match the native session" wording (Task F1, CAUTIONS escape rule).
func TestStrandedHandoffBlockNamesRecoverEscape(t *testing.T) {
	repo, record, _ := strandedRecoveryRequiredRecord(t)
	req := handoffEditRequest(record, repo, "codex", "any-session", "")
	req.Tool = "Bash"
	req.Command = "agent-harness issueops heartbeat --id " + record.ID + " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) + " --host codex --session-id any-session --agent-id worker-1"
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" {
		t.Fatalf("stranded record must still fence exact-cycle mutation: %#v", got)
	}
	for _, want := range []string{"handoff recover", "--id " + shellGuidanceQuote(record.ID), "--action cancel"} {
		if !strings.Contains(got.Reason, want) {
			t.Fatalf("stranded block reason missing escape token %q: %s", want, got.Reason)
		}
	}
	for _, forbidden := range []string{"flags do not match the native session", "remain read-only and poll"} {
		if strings.Contains(got.Reason, forbidden) {
			t.Fatalf("stranded block reason still uses misleading wording %q: %s", forbidden, got.Reason)
		}
	}
}

// TestSupervisedFenceEscapeResolverPerSubState pins the resolver output for each
// fence sub-state so the guard message and the actual escape stay in sync
// (CAUTIONS "pin the guard message and the actual escape command with a test").
func TestSupervisedFenceEscapeResolverPerSubState(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	cases := []struct {
		name    string
		mutate  func(*IssueOpsRecord)
		wantSub []string
	}{
		{
			name: "recovery_required with cleanup tombstone",
			mutate: func(r *IssueOpsRecord) {
				r.ExecutionHandoff.State = handoff.StateRecoveryRequired
				r.ExecutionHandoff.CleanupOnly = &issueopsmodel.IssueOpsOrcaCleanupArtifact{Kind: "worktree", ID: "wt-1", Reason: "path mismatch"}
			},
			wantSub: []string{"handoff recover", "--action cancel --confirm", "finalize-cancel", "approve-cleanup"},
		},
		{
			name: "recovery_required no cleanup",
			mutate: func(r *IssueOpsRecord) {
				r.ExecutionHandoff.State = handoff.StateRecoveryRequired
			},
			wantSub: []string{"handoff recover", "reconcile", "cancel", "abandon", "retry"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := record
			r.ExecutionHandoff = cloneLifecycleHandoffForTest(t, record.ExecutionHandoff)
			tc.mutate(&r)
			r.Repo = repo
			escape := supervisedFenceRecoverEscape(r)
			for _, want := range tc.wantSub {
				if !strings.Contains(escape, want) {
					t.Fatalf("%s escape missing %q: %s", tc.name, want, escape)
				}
			}
			if !strings.Contains(escape, record.ID) {
				t.Fatalf("%s escape lost exact id: %s", tc.name, escape)
			}
		})
	}
}

// TestStrandedHandoffOutOfWhitelistNamesEscape confirms an out-of-whitelist
// lifecycle command against the stranded record surfaces the escape rather than
// the identity-mismatch wording.
func TestStrandedHandoffOutOfWhitelistNamesEscape(t *testing.T) {
	repo, record, _ := strandedRecoveryRequiredRecord(t)
	req := handoffEditRequest(record, repo, "codex", "any-session", "")
	req.Tool = "Bash"
	// heartbeat is a lifecycle command that is not in the allowlist for a
	// recovery_required record from a non-worker session.
	req.Command = "agent-harness issueops heartbeat --id " + record.ID + " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) + " --host codex --session-id any-session --agent-id worker-1"
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" {
		t.Fatalf("out-of-whitelist lifecycle command must block: %#v", got)
	}
	if !strings.Contains(got.Reason, "handoff recover") {
		t.Fatalf("out-of-whitelist block must name the recover escape: %s", got.Reason)
	}
	if strings.Contains(got.Reason, "do not match the native session and persisted fence") {
		t.Fatalf("out-of-whitelist block still uses identity-mismatch wording: %s", got.Reason)
	}
}
