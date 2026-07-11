package issueops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
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

func TestHandoffRetryPreservesBoundedPriorAttemptAudit(t *testing.T) {
	t.Run("worker failed result and cleanup", func(t *testing.T) {
		stateRoot, record, _ := dispatchedHandoffRecord(t)
		record.ExecutionHandoff.State = handoff.StateClosed
		record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
		record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-old", AgentID: "agent-old"}
		record.ExecutionHandoff.Orca.TerminalBaselinePTYIDs = []string{"pty-baseline"}
		record.ExecutionHandoff.Orca.WorkerPTYID = "pty-old"
		record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-old"
		record.ExecutionHandoff.Orca.TaskID = "task-old"
		record.ExecutionHandoff.Orca.DispatchID = "dispatch-old"
		record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
		record.ExecutionHandoff.Result.CleanupReceipts = []string{"old worker resources stopped"}
		if _, err := WriteIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "retry", Confirm: true,
		}, nil, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-new", nil }}); err != nil {
			t.Fatal(err)
		}
		assertPriorAttemptJSON(t, stateRoot, record.ID, handoff.DispositionWorkerFailed, record.ExecutionHandoff.Orca, "pty-old", "term-old", "task-old", "dispatch-old", "old worker resources stopped", "")
	})

	t.Run("forced cancel failure", func(t *testing.T) {
		stateRoot, record, _ := dispatchedHandoffRecord(t)
		if _, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(record)); err != nil {
			t.Fatal(err)
		}
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "cancel", Confirm: true, Force: true, Reason: "worker lease proved stale",
		}, nil, handoffPrepareTestClock()); err != nil {
			t.Fatal(err)
		}
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "retry", Confirm: true,
		}, nil, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-new", nil }}); err != nil {
			t.Fatal(err)
		}
		assertPriorAttemptJSON(t, stateRoot, record.ID, handoff.DispositionCancelled, record.ExecutionHandoff.Orca, "pty-1", "term-1", "task-1", "dispatch-1", "", "forced_claimed_cancel")
	})
}

func assertPriorAttemptJSON(t *testing.T, stateRoot, id, disposition string, oldOrca *IssueOpsOrcaIdentity, pty, mailbox, task, dispatch, cleanup, failure string) {
	t.Helper()
	var raw struct {
		ExecutionHandoff struct {
			Attempt       int `json:"attempt"`
			PriorAttempts []struct {
				Attempt           int    `json:"attempt"`
				ClosedDisposition string `json:"closed_disposition"`
				Orca              struct {
					WorktreeID          string `json:"worktree_id"`
					WorktreeInstanceID  string `json:"worktree_instance_id"`
					WorktreePath        string `json:"worktree_path"`
					WorkerPTYID         string `json:"worker_pty_id"`
					WorkerMailboxHandle string `json:"worker_mailbox_handle"`
					TaskID              string `json:"task_id"`
					DispatchID          string `json:"dispatch_id"`
				} `json:"orca"`
				Result *struct {
					CleanupReceipts []string `json:"cleanup_receipts"`
				} `json:"result"`
				Failure *struct {
					Code string `json:"code"`
				} `json:"failure"`
			} `json:"prior_attempts"`
		} `json:"execution_handoff"`
	}
	if err := json.Unmarshal(rawIssueOpsBytesForTest(t, stateRoot, id), &raw); err != nil {
		t.Fatal(err)
	}
	if raw.ExecutionHandoff.Attempt != 2 || len(raw.ExecutionHandoff.PriorAttempts) != 1 {
		t.Fatalf("retry must retain one bounded prior attempt: %#v", raw.ExecutionHandoff)
	}
	prior := raw.ExecutionHandoff.PriorAttempts[0]
	if prior.Attempt != 1 || prior.ClosedDisposition != disposition || prior.Orca.WorktreeID != oldOrca.WorktreeID || prior.Orca.WorktreeInstanceID != oldOrca.WorktreeInstanceID || prior.Orca.WorktreePath != oldOrca.WorktreePath || prior.Orca.WorkerPTYID != pty || prior.Orca.WorkerMailboxHandle != mailbox || prior.Orca.TaskID != task || prior.Orca.DispatchID != dispatch {
		t.Fatalf("prior attempt lost external identity: %#v", prior)
	}
	if cleanup != "" && (prior.Result == nil || len(prior.Result.CleanupReceipts) != 1 || prior.Result.CleanupReceipts[0] != cleanup) {
		t.Fatalf("prior attempt lost cleanup evidence: %#v", prior.Result)
	}
	if failure != "" && (prior.Failure == nil || prior.Failure.Code != failure) {
		t.Fatalf("prior attempt lost failure evidence: %#v", prior.Failure)
	}
}

func TestHandoffForceAbandonRequiresConfirmedCompleteAbsentInventoryAndNeverRetries(t *testing.T) {
	tests := []struct {
		name    string
		pending IssueOpsExecutionHandoffPendingOperation
		prepare func(IssueOpsRecord, *dispatchOrcaFake)
	}{
		{
			name: "worktree create",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationWorktreeCreate, StartedAt: "2026-07-11T00:50:00Z", BaselineWorktreeIDs: []string{"wt-old"},
			},
			prepare: func(_ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}}
			},
		},
		{
			name: "terminal create",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationTerminalCreate, StartedAt: "2026-07-11T00:50:00Z", BaselinePTYIDs: []string{"pty-old"},
			},
			prepare: func(_ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}}
			},
		},
		{
			name: "task create",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationTaskCreate, StartedAt: "2026-07-11T00:50:00Z", BaselineTaskIDs: []string{"task-old"},
			},
			prepare: func(_ IssueOpsRecord, client *dispatchOrcaFake) {
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}}
			},
		},
		{
			name: "dispatch",
			pending: IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationDispatch, StartedAt: "2026-07-11T00:50:00Z", ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject",
			},
			prepare: func(record IssueOpsRecord, client *dispatchOrcaFake) {
				record.ExecutionHandoff.Orca.TaskID = "task-1"
				client.dispatchShowErr = &port.OrcaError{Code: "not_found", Invoked: true}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			if tt.pending.Kind == handoff.OperationDispatch {
				record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
				record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-1"
				record.ExecutionHandoff.Orca.TaskID = "task-1"
			}
			setRecoveryRequiredForTest(&record, tt.pending)
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			tt.prepare(record, client)
			got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "abandon", Confirm: true, Force: true, Reason: "complete inventory proves the invoked operation created no artifact",
			}, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if got.State != handoff.StateClosed || got.Disposition != handoff.DispositionCancelled || got.RecoveryCode != "force_abandoned_absent_operation" {
				t.Fatalf("force abandon did not close the absent operation: %#v", got)
			}
			before := append([]string(nil), client.trace...)
			if started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock()); err != nil || started.State != handoff.StateClosed {
				t.Fatalf("abandoned attempt start must be an inert terminal projection: %#v err=%v", started, err)
			}
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "retry", Confirm: true,
			}, client, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "must-not-be-used", nil }}); err == nil {
				t.Fatal("force-abandoned ambiguous operation must require a fresh cycle")
			}
			if strings.Join(client.trace, "\n") != strings.Join(before, "\n") || client.terminalCreates != 0 || client.dispatchCalls != 0 {
				t.Fatalf("abandoned operation was retried: trace before=%v after=%v creates=%d dispatch=%d", before, client.trace, client.terminalCreates, client.dispatchCalls)
			}
		})
	}
}

func TestHandoffForceAbandonRejectsMissingAuthorityAndYoungJournal(t *testing.T) {
	tests := []struct {
		name    string
		request IssueOpsHandoffRecoverRequest
		started string
		want    string
	}{
		{name: "confirmation", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Force: true, Reason: "complete inventory proves absence"}, started: "2026-07-11T00:50:00Z", want: "abandon requires --confirm"},
		{name: "force", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Reason: "complete inventory proves absence"}, started: "2026-07-11T00:50:00Z", want: "abandon requires --force"},
		{name: "reason", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Force: true}, started: "2026-07-11T00:50:00Z", want: "abandon requires a nonempty --reason"},
		{name: "reason bound", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Force: true, Reason: strings.Repeat("x", 4097)}, started: "2026-07-11T00:50:00Z", want: "abandon reason exceeds 4096 bytes"},
		{name: "minimum age", request: IssueOpsHandoffRecoverRequest{Action: "abandon", Confirm: true, Force: true, Reason: "complete inventory proves absence"}, started: "2026-07-11T00:57:04Z", want: "pending operation must be at least 5m0s old"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationWorktreeCreate, StartedAt: tt.started, BaselineWorktreeIDs: []string{"wt-old"},
			})
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			client := handoffDispatchFake(record)
			client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}}
			tt.request.ID = record.ID
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, tt.request, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("force abandon error = %v, want %q", err, tt.want)
			}
			after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if string(after) != string(before) || len(client.trace) != 0 {
				t.Fatalf("rejected force abandon changed state or inspected Orca: trace=%v", client.trace)
			}
		})
	}
}

func TestHandoffForceAbandonRequiresAuthoritativeAbsenceAndExactFence(t *testing.T) {
	tests := []struct {
		name    string
		pending IssueOpsExecutionHandoffPendingOperation
		prepare func(string, IssueOpsRecord, *dispatchOrcaFake)
		want    string
	}{
		{
			name: "worktree delta", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}, {ID: "wt-new"}}
			},
			want: "worktree inventory contains 1 post-baseline artifact",
		},
		{
			name: "worktree truncated inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktreeListErr = fmt.Errorf("Orca worktree list is incomplete")
			},
			want: "Orca worktree list is incomplete",
		},
		{
			name: "worktree unidentified row", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}, {}}
			},
			want: "worktree inventory contains missing or duplicate identities",
		},
		{
			name: "terminal delta", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}, {PTYID: "pty-new"}}
			},
			want: "terminal inventory contains 1 post-baseline artifact",
		},
		{
			name: "terminal truncated inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminalListErr = fmt.Errorf("Orca terminal list is incomplete")
			},
			want: "Orca terminal list is incomplete",
		},
		{
			name: "terminal unidentified row", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.terminals = []port.OrcaTerminal{{PTYID: "pty-old"}, {}}
			},
			want: "terminal inventory contains missing or duplicate identities",
		},
		{
			name: "task delta", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}, {ID: "task-new", Status: "ready"}}
			},
			want: "task inventory contains 1 post-baseline artifact",
		},
		{
			name: "task truncated inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.taskListErr = fmt.Errorf("Orca task list is incomplete")
			},
			want: "Orca task list is incomplete",
		},
		{
			name: "task unidentified row", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"}},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.tasks = []port.OrcaTask{{ID: "task-old", Status: "ready"}, {Status: "ready"}}
			},
			want: "task inventory contains missing or duplicate identities",
		},
		{
			name: "dispatch exists", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject"},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) { client.dispatchShowErr = nil },
			want:    "dispatch still exists",
		},
		{
			name: "dispatch unknown error", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject"},
			prepare: func(_ string, _ IssueOpsRecord, client *dispatchOrcaFake) {
				client.dispatchShowErr = fmt.Errorf("transport unavailable")
			},
			want: "inspect dispatch absence",
		},
		{
			name: "record changed during inventory", pending: IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationWorktreeCreate, BaselineWorktreeIDs: []string{"wt-old"}},
			prepare: func(stateRoot string, record IssueOpsRecord, client *dispatchOrcaFake) {
				client.worktrees = []port.OrcaWorktree{{ID: "wt-old"}}
				client.beforeWorktreeList = func() {
					current, err := ReadIssueOps(stateRoot, record.ID)
					if err != nil {
						t.Fatal(err)
					}
					current.UpdatedAt = "2026-07-11T01:01:00Z"
					if _, err := WriteIssueOps(stateRoot, current); err != nil {
						t.Fatal(err)
					}
				}
			},
			want: "handoff changed during force-abandon inventory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			tt.pending.StartedAt = "2026-07-11T00:50:00Z"
			if tt.pending.Kind == handoff.OperationDispatch {
				record.ExecutionHandoff.Orca.TaskID = "task-1"
			}
			setRecoveryRequiredForTest(&record, tt.pending)
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			tt.prepare(stateRoot, record, client)
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "abandon", Confirm: true, Force: true, Reason: "inventory cannot authoritatively prove absence",
			}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("force abandon error = %v, want %q", err, tt.want)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil {
				t.Fatalf("rejected force abandon changed the journal: %#v", persisted.ExecutionHandoff)
			}
		})
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
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock())
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

func TestHandoffRetryReattestsLegacyCodexBypassWithoutChangingSealedOptions(t *testing.T) {
	stateRoot, record := gitBackedDispatchedHandoff(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
	record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "codex-worker"}
	record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
	record.ExecutionHandoff.ContextOptions = &model.IssueOpsExecutionHandoffContextOptions{
		WorkerScope: "preserve this exact worker scope", RequiredDocs: []string{"AGENTS.md"}, AllowCodexHookTrustBypass: false,
	}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
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
	options := retried.ExecutionHandoff.ContextOptions
	if options == nil || options.AllowCodexHookTrustBypass || options.WorkerScope != "preserve this exact worker scope" || len(options.RequiredDocs) != 1 || options.RequiredDocs[0] != "AGENTS.md" {
		t.Fatalf("retry must preserve delivery options but clear per-attempt attestation: %#v", options)
	}
	client := handoffDispatchFake(retried)
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{
		ID: retried.ID, Confirm: true, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true},
	}, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !started.CodexHookTrustBypassAttested || len(client.terminalRequests) != 1 || !client.terminalRequests[0].AllowCodexHookTrustBypass {
		t.Fatalf("legacy retry attestation did not authorize exactly one Codex launch: result=%#v terminal=%#v", started, client.terminalRequests)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if handoff.ContextVersion != 1 || persisted.ExecutionHandoff.ContextVersion != 1 {
		t.Fatalf("optional attestation must remain context version 1 for legacy retry compatibility: constant=%d persisted=%d", handoff.ContextVersion, persisted.ExecutionHandoff.ContextVersion)
	}
	if persisted.ExecutionHandoff.ContextOptions == nil || !persisted.ExecutionHandoff.ContextOptions.AllowCodexHookTrustBypass || persisted.ExecutionHandoff.ContextOptions.WorkerScope != "preserve this exact worker scope" {
		t.Fatalf("reattestation changed the sealed delivery contract: %#v", persisted.ExecutionHandoff.ContextOptions)
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

func TestHandoffRecoverDispatchRequiresDurableDeliveryIdentityAndDispatchedStatus(t *testing.T) {
	tests := []struct {
		name, expectedAssignee, deliveryMode, returnedAssignee, status, dispatchID string
	}{
		{name: "assignee mismatch", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-other", status: "dispatched", dispatchID: "dispatch-1"},
		{name: "failed status", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-1", status: "failed", dispatchID: "dispatch-1"},
		{name: "completed status", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-1", status: "completed", dispatchID: "dispatch-1"},
		{name: "missing dispatch id", expectedAssignee: "term-1", deliveryMode: "inject", returnedAssignee: "term-1", status: "dispatched"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.Orca.TaskID = "task-1"
			record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-stale"
			setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
				Kind: handoff.OperationDispatch, ExpectedAssigneeHandle: tt.expectedAssignee, DeliveryMode: tt.deliveryMode,
			})
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			client.dispatch.ID = tt.dispatchID
			client.dispatch.AssigneeHandle = tt.returnedAssignee
			client.dispatch.Status = tt.status
			client.dispatch.Injected = false
			if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
				ID: record.ID, Action: "reconcile",
			}, client, handoffPrepareTestClock()); err == nil {
				t.Fatal("dispatch reconciliation must match its durable delivery identity and exact dispatched status")
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
	client := handoffDispatchFake()
	for _, tt := range []struct{ name, expectedAssignee, deliveryMode string }{
		{name: "missing expected assignee", deliveryMode: "inject"},
		{name: "missing delivery mode", expectedAssignee: "term-1"},
		{name: "unsupported delivery mode", expectedAssignee: "term-1", deliveryMode: "terminal_send"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", tt.expectedAssignee, tt.deliveryMode, client); err == nil {
				t.Fatal("invalid durable delivery journal was accepted")
			}
		})
	}
}

func TestHandoffRecoverDispatchUsesDurableRefreshedAssigneeWithoutInjectedField(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-stale"
	record.ExecutionHandoff.Orca.TaskID = "task-1"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.refreshedTerminal = port.OrcaTerminal{
		Handle: "term-live", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true,
	}
	client.dispatchErr = &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true}
	client.dispatch.AssigneeHandle = "term-live"
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("expected ambiguous dispatch")
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	operation := pending.ExecutionHandoff.PendingOperation
	if operation == nil || operation.Kind != handoff.OperationDispatch || operation.ExpectedAssigneeHandle != "term-live" || operation.DeliveryMode != "inject" {
		t.Fatalf("dispatch journal did not seal refreshed delivery identity: %#v", operation)
	}
	client.dispatchErr = nil
	client.dispatch = port.OrcaDispatch{ID: "dispatch-live", TaskID: "task-1", AssigneeHandle: "term-live", Status: "dispatched"}
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || got.Orca == nil || got.Orca.DispatchID != "dispatch-live" || got.Orca.WorkerMailboxHandle != "term-live" {
		t.Fatalf("installed-shape dispatch was not reconciled: %#v", got)
	}
}
