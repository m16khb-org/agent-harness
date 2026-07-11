package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestHandoffCancelClosesBeforeCleanup(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	updated, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != handoff.StateClosed || updated.Disposition != handoff.DispositionCancelled {
		t.Fatalf("cancel must close durably: %#v", updated)
	}
}

func TestHandoffCancelRejectsClaimedWithoutStaleEvidence(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true,
	}, nil, handoffPrepareTestClock()); err == nil {
		t.Fatal("claimed handoff cancel must require explicit stale or force evidence")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateClaimed || persisted.ExecutionHandoff.ClosedDisposition != "" {
		t.Fatalf("rejected claimed cancel mutated the handoff: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffCancelRejectsAmbiguousOperationBeforeClosing(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"},
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true,
	}, nil, handoffPrepareTestClock()); err == nil {
		t.Fatal("cancel must reject an unresolved external-operation journal")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.ClosedDisposition != "" || persisted.ExecutionHandoff.PendingOperation == nil {
		t.Fatalf("rejected cancel must preserve recovery state and journal: %#v", persisted.ExecutionHandoff)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, nil, IssueOpsHandoffPrepareClock{
		Now: handoffPrepareTestClock().Now,
		NewEpoch: func() (string, error) {
			return "must-not-be-used", nil
		},
	}); err == nil {
		t.Fatal("retry must reject a cancelled attempt with an unresolved journal")
	}
}

func TestHandoffForcedClaimedCancelPersistsEvidenceAndFencesWorker(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "worker heartbeat lost after coordinator investigation",
	}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateClosed || persisted.ExecutionHandoff.ClosedDisposition != handoff.DispositionCancelled || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "forced_claimed_cancel" || persisted.ExecutionHandoff.Failure.Message == "" {
		t.Fatalf("forced cancellation evidence not persisted: %#v", persisted.ExecutionHandoff)
	}
	if _, err := RecordIssueOpsHeartbeatWithRequest(stateRoot, IssueOpsHeartbeatRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID,
	}); err == nil {
		t.Fatal("closed claimed fence must not heartbeat")
	}
	if _, err := FinishIssueOpsHandoff(stateRoot, IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeFailed,
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}); err == nil {
		t.Fatal("closed claimed fence must not finish")
	}
}

func TestHandoffForcedCancelRedactsReasonBeforeStateAndProjection(t *testing.T) {
	tests := []struct {
		name, value, reason string
	}{
		{name: "authorization bearer", value: "opaque-bearer-value-7F3A", reason: "Authorization: Bearer opaque-bearer-value-7F3A"},
		{name: "api key", value: "opaque-api-value-91C2", reason: "api_key=opaque-api-value-91C2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			claim := handoffClaimRequest(record)
			if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
				t.Fatal(err)
			}
			result, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: tt.reason,
			}, nil, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, encoded := range []string{string(rawIssueOpsBytesForTest(t, stateRoot, record.ID)), fmt.Sprintf("%#v", persisted), fmt.Sprintf("%#v", result)} {
				if strings.Contains(encoded, tt.value) {
					t.Fatalf("forced-cancel state/projection leaked %s value: %s", tt.name, encoded)
				}
			}
		})
	}
}

func TestHandoffRetryUsesNewAttemptAndEpoch(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "retry", Confirm: true}, nil, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-2", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempt != 2 || got.State != handoff.StateCoordinatorPreparing {
		t.Fatalf("retry did not create new fenced attempt: %#v", got)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.ExecutionHandoff.OwnershipEpoch != "epoch-2" {
		t.Fatalf("epoch not replaced: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffRetryAllowsWorkerFailedDisposition(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
	record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "codex-worker"}
	record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, nil, IssueOpsHandoffPrepareClock{
		Now: handoffPrepareTestClock().Now,
		NewEpoch: func() (string, error) {
			return "epoch-worker-failed", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempt != 2 || got.State != handoff.StateCoordinatorPreparing {
		t.Fatalf("worker_failed retry did not create a new fenced attempt: %#v", got)
	}
}

func TestHandoffRetryPinsCleanPartialCommitAsNewAttemptBase(t *testing.T) {
	stateRoot, record := gitBackedDispatchedHandoff(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "internal/partial.go", "package internal\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: partial worker result"}} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	partialHead := preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD")
	if _, err := FinishIssueOpsHandoff(stateRoot, IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeFailed,
		Verification: []string{"focused test failed after partial commit"}, CleanupReceipts: []string{"worker resources stopped"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, nil, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-2", nil }}); err != nil {
		t.Fatal(err)
	}
	retried, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.ExecutionHandoff.AttemptBaseHead != partialHead {
		t.Fatalf("retry attempt base = %q, want partial HEAD %q", retried.ExecutionHandoff.AttemptBaseHead, partialHead)
	}
	client := handoffDispatchFake(retried)
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := ReadIssueOps(stateRoot, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(dispatched)); err != nil {
		t.Fatalf("new worker claim at the clean retry checkpoint failed: %v", err)
	}
}

func TestHandoffRetryRejectsUnsafeWorktreeCheckpointBeforeNewAttempt(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, IssueOpsRecord)
	}{
		{name: "dirty worktree", mutate: func(t *testing.T, record IssueOpsRecord) {
			writeIssueOpsFile(t, record.WorktreePath, "dirty.txt", "dirty\n")
		}},
		{name: "branch mismatch", mutate: func(t *testing.T, record IssueOpsRecord) {
			if code, _, stderr := preflight.GitCmd(record.WorktreePath, "switch", "-q", "-c", "other-branch"); code != 0 {
				t.Fatalf("git switch failed: %s", stderr)
			}
		}},
		{name: "head lookup failure", mutate: func(t *testing.T, record IssueOpsRecord) {
			ref := filepath.Join(record.WorktreePath, ".git", "refs", "heads", record.Branch)
			if err := os.Remove(ref); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := gitBackedDispatchedHandoff(t)
			record.ExecutionHandoff.State = handoff.StateClosed
			record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
			record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "codex-worker"}
			record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			tt.mutate(t, record)
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "retry", Confirm: true,
			}, nil, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "must-not-be-used", nil }}); err == nil {
				t.Fatal("unsafe retry checkpoint must fail before a new attempt")
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateClosed || persisted.ExecutionHandoff.Attempt != 1 {
				t.Fatalf("rejected retry mutated the attempt: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffRetryRejectsAcceptedDisposition(t *testing.T) {
	stateRoot, record, claim, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	if _, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
		ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, nil, IssueOpsHandoffPrepareClock{
		Now: handoffPrepareTestClock().Now,
		NewEpoch: func() (string, error) {
			return "must-not-be-used", nil
		},
	}); err == nil {
		t.Fatal("accepted handoff must remain terminal and reject retry")
	}

	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := persisted.ExecutionHandoff
	if got.State != handoff.StateClosed || got.ClosedDisposition != handoff.DispositionAccepted || got.Attempt != 1 || got.OwnershipEpoch != record.ExecutionHandoff.OwnershipEpoch {
		t.Fatalf("rejected accepted retry mutated terminal handoff: %#v", got)
	}
}

func TestHandoffRecoverExactOneOnlyAndNeverAdvances(t *testing.T) {
	tests := []struct {
		name string
		rows []port.OrcaTerminal
		ok   bool
	}{
		{name: "zero"},
		{name: "one", rows: []port.OrcaTerminal{{Handle: "term-new", PTYID: "pty-new", WorktreeID: "wt-1", WorktreePath: "WORKER_ROOT", Connected: true, Writable: true}}, ok: true},
		{name: "multiple", rows: []port.OrcaTerminal{{Handle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1"}, {Handle: "term-2", PTYID: "pty-2", WorktreeID: "wt-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"old"}})
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			for i := range tt.rows {
				if tt.rows[i].WorktreePath == "WORKER_ROOT" {
					tt.rows[i].WorktreePath = record.WorktreePath
				}
			}
			client := handoffDispatchFake(record)
			client.terminals = tt.rows
			got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock())
			if tt.ok && (err != nil || got.State != handoff.StateCoordinatorPreparing || got.NextCommand == "") {
				t.Fatalf("expected reconciled status only, got %#v err=%v", got, err)
			}
			if tt.ok && (client.taskCreates != 0 || client.dispatchCalls != 0) {
				t.Fatal("reconcile advanced to a next operation")
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected fail closed, got %#v", got)
			}
		})
	}
}

func TestHandoffRecoverDispatchRequiresPersistedAssigneeAndInjection(t *testing.T) {
	tests := []struct {
		name            string
		persistedHandle string
		assignee        string
		injected        bool
	}{
		{name: "missing persisted assignee", assignee: "term-1", injected: true},
		{name: "assignee mismatch", persistedHandle: "term-1", assignee: "term-other", injected: true},
		{name: "not injected", persistedHandle: "term-1", assignee: "term-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.Orca.TaskID = "task-1"
			record.ExecutionHandoff.Orca.WorkerMailboxHandle = tt.persistedHandle
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch})
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			client.dispatch.AssigneeHandle = tt.assignee
			client.dispatch.Injected = tt.injected
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "reconcile",
			}, client, handoffPrepareTestClock()); err == nil {
				t.Fatal("dispatch reconciliation must match the persisted assignee and injected delivery")
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil {
				t.Fatalf("failed dispatch reconciliation advanced state: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}
