package handoff

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestIssueOpsExecutionHandoffTransitionTable(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-handoff", Repo: "/repo", Branch: "16-demo", WorktreePath: "/repo.worktrees/16-demo"}
	prepared, err := Prepare(record, PrepareRequest{
		Attempt:         1,
		OwnershipEpoch:  "epoch-1",
		AttemptBaseHead: strings.Repeat("0", 40),
		CoordinatorRoot: "/repo",
		WorkerRoot:      "/repo.worktrees/16-demo",
		Agent:           "codex",
		Now:             "2026-07-11T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := prepared.ExecutionHandoff.State; got != StateCoordinatorPreparing {
		t.Fatalf("prepared state = %q, want %q", got, StateCoordinatorPreparing)
	}

	contextSHA := strings.Repeat("a", 64)
	prepared, err = SetContext(prepared, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, 1, contextSHA, strings.Repeat("d", 64), model.IssueOpsExecutionHandoffContextOptions{}, "2026-07-11T00:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := Dispatch(prepared, Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA}, model.IssueOpsOrcaIdentity{
		RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo",
		WorktreeID: "worktree-1", WorktreeInstanceID: "instance-1", WorktreePath: "/repo.worktrees/16-demo",
		WorkerPTYID: "pty-1", WorkerMailboxHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1",
	}, "2026-07-11T00:02:00Z")
	if err != nil {
		t.Fatal(err)
	}

	worker := model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"}
	claimed, err := Claim(dispatched, ClaimRequest{
		Fence:      Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA},
		Worker:     worker,
		WorkerRoot: "/repo.worktrees/16-demo",
		Now:        "2026-07-11T00:03:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := claimed.ExecutionHandoff.State; got != StateClaimed {
		t.Fatalf("claimed state = %q, want %q", got, StateClaimed)
	}

	result := model.IssueOpsExecutionHandoffResult{
		Outcome:          OutcomeCompleted,
		FinalHead:        strings.Repeat("b", 40),
		ChangedFiles:     []string{"internal/core/issueops/example.go"},
		TuringReportPath: ".agent-harness/research/report.md",
		Verification:     []string{"go test ./...: pass"},
		CleanupReceipts:  []string{"cleanup: no runtime resources spawned"},
		TaskID:           "task-1",
		DispatchID:       "dispatch-1",
	}
	submitted, err := Finish(claimed, FinishRequest{
		Fence:  Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA},
		Worker: worker,
		Result: result,
		Now:    "2026-07-11T00:04:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := submitted.ExecutionHandoff.State; got != StateSubmitted {
		t.Fatalf("submitted state = %q, want %q", got, StateSubmitted)
	}

	accepted, err := Accept(submitted, AcceptRequest{
		Fence:     Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA},
		FinalHead: result.FinalHead,
		Now:       "2026-07-11T00:05:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := accepted.ExecutionHandoff.State; got != StateClosed {
		t.Fatalf("accepted state = %q, want %q", got, StateClosed)
	}
	if got := accepted.ExecutionHandoff.ClosedDisposition; got != DispositionAccepted {
		t.Fatalf("accepted disposition = %q, want %q", got, DispositionAccepted)
	}
}

func TestIssueOpsExecutionHandoffRejectsStaleAttemptEpochAndContext(t *testing.T) {
	record := dispatchedRecordForTest(t)
	tests := []struct {
		name  string
		fence Fence
	}{
		{name: "attempt", fence: Fence{Attempt: 2, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}},
		{name: "epoch", fence: Fence{Attempt: 1, OwnershipEpoch: "epoch-stale", ContextSHA256: strings.Repeat("a", 64)}},
		{name: "context", fence: Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("c", 64)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Claim(record, ClaimRequest{
				Fence:      tt.fence,
				Worker:     model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"},
				WorkerRoot: "/repo.worktrees/16-demo",
			})
			if err == nil || !strings.Contains(err.Error(), "stale handoff") {
				t.Fatalf("Claim() error = %v, want stale handoff", err)
			}
		})
	}
}

func TestIssueOpsExecutionHandoffEnvelopeRejectsCorruption(t *testing.T) {
	valid := dispatchedRecordForTest(t)
	tests := []struct {
		name   string
		mutate func(*model.IssueOpsExecutionHandoff)
	}{
		{name: "zero protocol", mutate: func(h *model.IssueOpsExecutionHandoff) { h.ProtocolVersion = 0 }},
		{name: "future protocol", mutate: func(h *model.IssueOpsExecutionHandoff) { h.ProtocolVersion = ProtocolVersion + 1 }},
		{name: "wrong driver", mutate: func(h *model.IssueOpsExecutionHandoff) { h.Driver = "inline" }},
		{name: "zero attempt", mutate: func(h *model.IssueOpsExecutionHandoff) { h.Attempt = 0 }},
		{name: "empty epoch", mutate: func(h *model.IssueOpsExecutionHandoff) { h.OwnershipEpoch = "" }},
		{name: "empty coordinator root", mutate: func(h *model.IssueOpsExecutionHandoff) { h.CoordinatorRoot = "" }},
		{name: "unknown state", mutate: func(h *model.IssueOpsExecutionHandoff) { h.State = "future_state" }},
		{name: "dispatched disposition", mutate: func(h *model.IssueOpsExecutionHandoff) { h.ClosedDisposition = DispositionCancelled }},
		{name: "dispatched pending", mutate: func(h *model.IssueOpsExecutionHandoff) {
			h.PendingOperation = &model.IssueOpsExecutionHandoffPendingOperation{Kind: OperationDispatch}
		}},
		{name: "closed without disposition", mutate: func(h *model.IssueOpsExecutionHandoff) { h.State = StateClosed }},
		{name: "recovery without pending", mutate: func(h *model.IssueOpsExecutionHandoff) { h.State = StateRecoveryRequired }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := cloneRecord(t, valid)
			tt.mutate(record.ExecutionHandoff)
			if err := ValidateEnvelope(record); err == nil {
				t.Fatalf("ValidateEnvelope() accepted %#v", record.ExecutionHandoff)
			}
			if _, err := Claim(record, ClaimRequest{Fence: Fence{Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256}, Worker: model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "s"}, WorkerRoot: record.ExecutionHandoff.WorkerRoot}); err == nil {
				t.Fatal("Claim() must fail closed on an invalid envelope")
			}
		})
	}
}

func TestIssueOpsExecutionHandoffEnvelopeRejectsStateSpecificCorruption(t *testing.T) {
	tests := []struct {
		name   string
		valid  func(*testing.T) model.IssueOpsRecord
		mutate func(*model.IssueOpsRecord)
	}{
		{name: "worker root not canonical flat path", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.WorkerRoot = "/repo.worktrees/nested/16-demo" }},
		{name: "relative repo", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) {
			r.Repo = "repo"
			r.ExecutionHandoff.CoordinatorRoot = "repo"
			r.ExecutionHandoff.WorkerRoot = "repo.worktrees/16-demo"
			r.WorktreePath = "repo.worktrees/16-demo"
			r.ExecutionHandoff.Orca.WorktreePath = "repo.worktrees/16-demo"
		}},
		{name: "relative coordinator root", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.CoordinatorRoot = "repo" }},
		{name: "relative worker root", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.WorkerRoot = "repo.worktrees/16-demo" }},
		{name: "record worktree mismatches worker root", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.WorktreePath += "-other" }},
		{name: "Orca path mismatches worker root", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Orca.WorktreePath += "-other" }},
		{name: "malformed context hash", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.ContextSHA256 = strings.Repeat("z", 64) }},
		{name: "partial sealed context", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.ContextSourceSHA256 = "" }},
		{name: "padded ownership epoch", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.OwnershipEpoch = " epoch-1 " }},
		{name: "padded agent", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Agent = " codex " }},
		{name: "incomplete stable terminal identity", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Orca.WorkerTabID = "tab-only" }},
		{name: "dispatched stale worker session", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) {
			r.ExecutionHandoff.WorkerSession = &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session"}
		}},
		{name: "dispatched stale failure", valid: dispatchedRecordForTest, mutate: func(r *model.IssueOpsRecord) {
			r.ExecutionHandoff.Failure = &model.IssueOpsExecutionHandoffFailure{Code: "stale", Message: "stale"}
		}},
		{name: "claimed padded session", valid: claimedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.WorkerSession.SessionID = " session-1 " }},
		{name: "claimed unsupported host", valid: claimedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.WorkerSession.Host = "reasonix" }},
		{name: "submitted missing report", valid: submittedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Result.TuringReportPath = "" }},
		{name: "submitted unsafe changed file", valid: submittedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Result.ChangedFiles = []string{"../source.go"} }},
		{name: "submitted whitespace receipt", valid: submittedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Result.CleanupReceipts = []string{"   "} }},
		{name: "submitted task mismatch", valid: submittedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Result.TaskID = "task-other" }},
		{name: "submitted stale failure", valid: submittedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) {
			r.ExecutionHandoff.Failure = &model.IssueOpsExecutionHandoffFailure{Code: "stale", Message: "stale"}
		}},
		{name: "accepted cleanup-only stale field", valid: acceptedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) {
			r.ExecutionHandoff.CleanupOnly = &model.IssueOpsOrcaCleanupArtifact{Kind: "worktree", ID: "wt-x", InstanceID: "inst-x", Path: "/tmp/x", Reason: "stale"}
		}},
		{name: "worker failed missing session", valid: failedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.WorkerSession = nil }},
		{name: "worker failed dispatch mismatch", valid: failedRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Result.DispatchID = "dispatch-other" }},
		{name: "recovery missing failure", valid: recoveryRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.Failure = nil }},
		{name: "recovery padded operation", valid: recoveryRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.PendingOperation.Kind = " worktree_create " }},
		{name: "preparing stale result", valid: preparingRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) {
			r.ExecutionHandoff.Result = &model.IssueOpsExecutionHandoffResult{Outcome: OutcomeFailed}
		}},
		{name: "preparing stale claimed timestamp", valid: preparingRecordForEnvelopeTest, mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.ClaimedAt = "2026-07-11T00:00:00Z" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := tt.valid(t)
			if err := ValidateEnvelope(record); err != nil {
				t.Fatalf("valid fixture is invalid before corruption: %v", err)
			}
			tt.mutate(&record)
			if err := ValidateEnvelope(record); err == nil {
				t.Fatalf("corrupted envelope was accepted: %#v", record.ExecutionHandoff)
			}
		})
	}
}

func TestIssueOpsExecutionHandoffEnvelopeAllowsCanonicalCancelledPredecessorEvidence(t *testing.T) {
	tests := []struct {
		name   string
		valid  func(*testing.T) model.IssueOpsRecord
		mutate func(*model.IssueOpsExecutionHandoff)
	}{
		{name: "preparing", valid: preparingRecordForEnvelopeTest},
		{name: "dispatched", valid: dispatchedRecordForTest},
		{name: "claimed", valid: claimedRecordForEnvelopeTest, mutate: func(h *model.IssueOpsExecutionHandoff) {
			h.Failure = &model.IssueOpsExecutionHandoffFailure{Code: "forced_claimed_cancel", Message: "worker stale", At: "2026-07-11T00:00:00Z"}
		}},
		{name: "submitted", valid: submittedRecordForEnvelopeTest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := tt.valid(t)
			record.ExecutionHandoff.State = StateClosed
			record.ExecutionHandoff.ClosedDisposition = DispositionCancelled
			if tt.mutate != nil {
				tt.mutate(record.ExecutionHandoff)
			}
			if err := ValidateEnvelope(record); err != nil {
				t.Fatalf("canonical cancelled predecessor evidence was rejected: %v", err)
			}
			record.ExecutionHandoff.AcceptedAt = "2026-07-11T00:00:00Z"
			if err := ValidateEnvelope(record); err == nil {
				t.Fatal("cancelled handoff must reject accepted_at")
			}
		})
	}
}

func TestIssueOpsExecutionHandoffPendingOperationSurvivesRoundTrip(t *testing.T) {
	record, err := Prepare(model.IssueOpsRecord{ID: "io-handoff", Repo: "/repo", Branch: "16-demo", WorktreePath: "/repo.worktrees/16-demo"}, PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-1", AttemptBaseHead: strings.Repeat("0", 40), CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/16-demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := model.IssueOpsExecutionHandoffPendingOperation{
		Kind:           OperationTerminalCreate,
		StartedAt:      "2026-07-11T00:00:00Z",
		BaselinePTYIDs: []string{"pty-a", "pty-b"},
	}
	updated, err := BeginOperation(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, pending)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip := cloneRecord(t, updated)
	if !reflect.DeepEqual(roundTrip.ExecutionHandoff.PendingOperation, &pending) {
		t.Fatalf("pending operation round trip = %#v, want %#v", roundTrip.ExecutionHandoff.PendingOperation, pending)
	}
}

func TestFinishIdempotencyRequiresOriginalWorker(t *testing.T) {
	record := dispatchedRecordForTest(t)
	fence := Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}
	worker := model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"}
	claimed, err := Claim(record, ClaimRequest{Fence: fence, Worker: worker, WorkerRoot: "/repo.worktrees/16-demo"})
	if err != nil {
		t.Fatal(err)
	}
	result := model.IssueOpsExecutionHandoffResult{
		Outcome: OutcomeCompleted, FinalHead: strings.Repeat("b", 40),
		ChangedFiles:     []string{".agent-harness/research/report.md"},
		TuringReportPath: ".agent-harness/research/report.md",
		Verification:     []string{"go test ./...: pass"}, CleanupReceipts: []string{"no worker resources"}, TaskID: "task-1", DispatchID: "dispatch-1",
	}
	submitted, err := Finish(claimed, FinishRequest{Fence: fence, Worker: worker, Result: result})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Finish(submitted, FinishRequest{Fence: fence, Worker: worker, Result: result}); err != nil {
		t.Fatalf("same worker identical finish must remain idempotent: %v", err)
	}
	other := worker
	other.SessionID = "session-other"
	if _, err := Finish(submitted, FinishRequest{Fence: fence, Worker: other, Result: result}); err == nil {
		t.Fatal("different worker must not receive an idempotent finish success")
	}
}

func TestFinishRejectsWhitespaceOnlyCompletedEvidence(t *testing.T) {
	record := dispatchedRecordForTest(t)
	fence := Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}
	worker := model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
	claimed, err := Claim(record, ClaimRequest{Fence: fence, Worker: worker, WorkerRoot: "/repo.worktrees/16-demo"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name         string
		verification []string
		cleanup      []string
	}{
		{name: "verification", verification: []string{"  \t "}, cleanup: []string{"stopped"}},
		{name: "cleanup", verification: []string{"go test: pass"}, cleanup: []string{" \n "}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Finish(claimed, FinishRequest{Fence: fence, Worker: worker, Result: model.IssueOpsExecutionHandoffResult{
				Outcome: OutcomeCompleted, FinalHead: strings.Repeat("b", 40), Verification: tt.verification, CleanupReceipts: tt.cleanup,
			}})
			if err == nil {
				t.Fatal("whitespace-only completed evidence must fail")
			}
		})
	}
}

func TestFinishRejectsPaddedCompletedOutcomeWithoutReport(t *testing.T) {
	record := dispatchedRecordForTest(t)
	fence := Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}
	worker := model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
	claimed, err := Claim(record, ClaimRequest{Fence: fence, Worker: worker, WorkerRoot: "/repo.worktrees/16-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Finish(claimed, FinishRequest{Fence: fence, Worker: worker, Result: model.IssueOpsExecutionHandoffResult{
		Outcome: " completed ", FinalHead: strings.Repeat("b", 40),
		Verification: []string{"go test: pass"}, CleanupReceipts: []string{"resources stopped"},
	}}); err == nil {
		t.Fatal("canonical completed outcome without a Turing report must fail")
	}
}

func TestHandoffStateRedactsFreeformEvidenceBeforeReturning(t *testing.T) {
	const bearer = "opaque-bearer-value-7F3A"
	const apiKey = "opaque-api-value-91C2"
	claimed := claimedRecordForEnvelopeTest(t)
	h := claimed.ExecutionHandoff
	worker := *h.WorkerSession
	fence := Fence{Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch, ContextSHA256: h.ContextSHA256}
	submitted, err := Finish(claimed, FinishRequest{Fence: fence, Worker: worker, Result: model.IssueOpsExecutionHandoffResult{
		Outcome: OutcomeCompleted, FinalHead: strings.Repeat("b", 40),
		ChangedFiles: []string{".agent-harness/research/report.md"}, TuringReportPath: ".agent-harness/research/report.md",
		Verification: []string{"Authorization: Bearer " + bearer}, CleanupReceipts: []string{"api_key=" + apiKey},
		EvidenceDigest: "Authorization: Bearer " + bearer, TaskID: h.Orca.TaskID, DispatchID: h.Orca.DispatchID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	assertNoHandoffSecret(t, submitted, bearer, apiKey)

	preparing := preparingRecordForEnvelopeTest(t)
	pending, err := BeginOperation(preparing, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: OperationWorktreeCreate, StartedAt: "2026-07-11T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := MarkRecoveryRequired(pending, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, model.IssueOpsExecutionHandoffFailure{
		Code: "create_failed", Message: "Authorization: Bearer " + bearer, At: "2026-07-11T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertNoHandoffSecret(t, recovery, bearer)

	cleanup, err := MarkCleanupOnlyWorktree(pending, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, model.IssueOpsOrcaCleanupArtifact{
		Kind: "worktree", ID: "wt-cleanup", InstanceID: "instance-1", Path: "/repo.worktrees/16-demo-invalid", Reason: "api_key=" + apiKey,
	}, model.IssueOpsExecutionHandoffFailure{Code: "worktree_cleanup_only", Message: "cleanup identity mismatch", At: "2026-07-11T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	assertNoHandoffSecret(t, cleanup, apiKey)
}

func TestFailureMessageIsOptionalAndBoundedListsRejectRawSecrets(t *testing.T) {
	record := recoveryRecordForEnvelopeTest(t)
	record.ExecutionHandoff.Failure.Message = ""
	if err := ValidateEnvelope(record); err != nil {
		t.Fatalf("optional failure message was rejected: %v", err)
	}
	if err := validateBoundedStringList([]string{"Authorization: Bearer opaque-bearer-value-7F3A"}, 8, 4096, 8192, true); err == nil {
		t.Fatal("bounded evidence list accepted an injected raw Bearer value")
	}
}

func assertNoHandoffSecret(t *testing.T, record model.IssueOpsRecord, secrets ...string) {
	t.Helper()
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("handoff state leaked secret %q: %s", secret, encoded)
		}
	}
}

func TestMarkRecoveryRequiredOnlyFromPendingCoordinatorOperation(t *testing.T) {
	prepared, err := Prepare(model.IssueOpsRecord{ID: "io-handoff", Repo: "/repo", Branch: "16-demo"}, PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-1", AttemptBaseHead: strings.Repeat("0", 40), CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/16-demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	fence := Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}
	failure := model.IssueOpsExecutionHandoffFailure{Code: "create_ambiguous", At: "2026-07-11T00:00:00Z"}
	if _, err := MarkRecoveryRequired(prepared, fence, failure); err == nil {
		t.Fatal("recovery without a pending operation must fail")
	}
	prepared, err = BeginOperation(prepared, fence, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: OperationWorktreeCreate, StartedAt: failure.At,
	})
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := MarkRecoveryRequired(prepared, fence, failure)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.ExecutionHandoff.State != StateRecoveryRequired || recovery.ExecutionHandoff.PendingOperation == nil {
		t.Fatalf("valid pending coordinator operation did not enter recovery: %#v", recovery.ExecutionHandoff)
	}
}

func TestMarkRecoveryRequiredNeverReopensTerminalOrWorkerStates(t *testing.T) {
	tests := []struct {
		name, state, disposition string
	}{
		{name: "dispatched", state: StateDispatched},
		{name: "claimed", state: StateClaimed},
		{name: "closed cancelled", state: StateClosed, disposition: DispositionCancelled},
		{name: "closed accepted", state: StateClosed, disposition: DispositionAccepted},
		{name: "closed worker failed", state: StateClosed, disposition: DispositionWorkerFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := dispatchedRecordForTest(t)
			record.ExecutionHandoff.State = tt.state
			record.ExecutionHandoff.ClosedDisposition = tt.disposition
			record.ExecutionHandoff.PendingOperation = &model.IssueOpsExecutionHandoffPendingOperation{Kind: OperationDispatch}
			updated, err := MarkRecoveryRequired(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, model.IssueOpsExecutionHandoffFailure{Code: "late_error"})
			if err == nil {
				t.Fatalf("%s must reject recovery reopening: %#v", tt.name, updated.ExecutionHandoff)
			}
			if record.ExecutionHandoff.State != tt.state || record.ExecutionHandoff.ClosedDisposition != tt.disposition {
				t.Fatalf("rejected recovery mutated input: %#v", record.ExecutionHandoff)
			}
		})
	}
}

func dispatchedRecordForTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record, err := Prepare(model.IssueOpsRecord{ID: "io-handoff", Repo: "/repo", Branch: "16-demo", WorktreePath: "/repo.worktrees/16-demo"}, PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-1", AttemptBaseHead: strings.Repeat("0", 40), CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/16-demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("a", 64)
	record, err = SetContext(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, 1, sha, strings.Repeat("d", 64), model.IssueOpsExecutionHandoffContextOptions{}, "")
	if err != nil {
		t.Fatal(err)
	}
	record, err = Dispatch(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: sha}, model.IssueOpsOrcaIdentity{
		RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo",
		WorktreeID: "worktree-1", WorktreeInstanceID: "instance-1", WorktreePath: "/repo.worktrees/16-demo",
		WorkerPTYID: "pty-1", WorkerMailboxHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func preparingRecordForEnvelopeTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record, err := Prepare(model.IssueOpsRecord{ID: "io-handoff", Repo: "/repo", Branch: "16-demo"}, PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-1", AttemptBaseHead: strings.Repeat("0", 40), CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/16-demo", Agent: "codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func recoveryRecordForEnvelopeTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record := preparingRecordForEnvelopeTest(t)
	var err error
	record, err = BeginOperation(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, model.IssueOpsExecutionHandoffPendingOperation{Kind: OperationWorktreeCreate, StartedAt: "2026-07-11T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = MarkRecoveryRequired(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, model.IssueOpsExecutionHandoffFailure{Code: "create_ambiguous", Message: "reconcile exact inventory", At: "2026-07-11T00:00:01Z"})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func claimedRecordForEnvelopeTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record := dispatchedRecordForTest(t)
	record, err := Claim(record, ClaimRequest{
		Fence:  Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)},
		Worker: model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"}, WorkerRoot: "/repo.worktrees/16-demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func submittedRecordForEnvelopeTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record := claimedRecordForEnvelopeTest(t)
	worker := *record.ExecutionHandoff.WorkerSession
	record, err := Finish(record, FinishRequest{
		Fence: Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}, Worker: worker,
		Result: model.IssueOpsExecutionHandoffResult{
			Outcome: OutcomeCompleted, FinalHead: strings.Repeat("b", 40), ChangedFiles: []string{"internal/x.go", ".agent-harness/research/report.md"},
			TuringReportPath: ".agent-harness/research/report.md", Verification: []string{"go test: pass"}, CleanupReceipts: []string{"worker resources stopped"},
			TaskID: "task-1", DispatchID: "dispatch-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func acceptedRecordForEnvelopeTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record := submittedRecordForEnvelopeTest(t)
	record, err := Accept(record, AcceptRequest{
		Fence: Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}, FinalHead: strings.Repeat("b", 40),
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func failedRecordForEnvelopeTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record := claimedRecordForEnvelopeTest(t)
	worker := *record.ExecutionHandoff.WorkerSession
	record, err := Finish(record, FinishRequest{
		Fence: Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}, Worker: worker,
		Result: model.IssueOpsExecutionHandoffResult{Outcome: OutcomeFailed, TaskID: "task-1", DispatchID: "dispatch-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func cloneRecord(t *testing.T, record model.IssueOpsRecord) model.IssueOpsRecord {
	t.Helper()
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var cloned model.IssueOpsRecord
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}
