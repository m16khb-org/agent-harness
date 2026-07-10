package issueops

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

func TestHandoffClaimRequiresMatchingWorkerTuple(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	base := handoffClaimRequest(record)
	tests := []struct {
		name   string
		mutate func(*IssueOpsHandoffClaimRequest)
	}{
		{name: "attempt", mutate: func(r *IssueOpsHandoffClaimRequest) { r.Attempt++ }},
		{name: "epoch", mutate: func(r *IssueOpsHandoffClaimRequest) { r.OwnershipEpoch = "stale" }},
		{name: "context", mutate: func(r *IssueOpsHandoffClaimRequest) { r.ContextSHA256 = "stale" }},
		{name: "session", mutate: func(r *IssueOpsHandoffClaimRequest) { r.SessionID = "" }},
		{name: "root", mutate: func(r *IssueOpsHandoffClaimRequest) { r.CWD = record.Repo }},
		{name: "worktree", mutate: func(r *IssueOpsHandoffClaimRequest) { r.OrcaWorktreeID = "wrong" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			tt.mutate(&req)
			if _, err := ClaimIssueOpsHandoff(stateRoot, req); err == nil {
				t.Fatal("expected fenced claim rejection")
			}
		})
	}
}

func TestHandoffClaimIsIdempotentForSameOwnerOnly(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	req := handoffClaimRequest(record)
	first, err := ClaimIssueOpsHandoff(stateRoot, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ClaimIssueOpsHandoff(stateRoot, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ExecutionHandoff.State != handoff.StateClaimed || second.ExecutionHandoff.WorkerSession.SessionID != req.SessionID {
		t.Fatalf("unexpected claim result: %#v", second.ExecutionHandoff)
	}
	other := req
	other.SessionID = "other"
	if _, err := ClaimIssueOpsHandoff(stateRoot, other); err == nil {
		t.Fatal("different owner must not steal claim")
	}
}

func TestHandoffHeartbeatFencesStaleWorker(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	req := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, req)
	if err != nil {
		t.Fatal(err)
	}
	hb := IssueOpsHeartbeatRequest{ID: claimed.ID, Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256, Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}
	updated, err := RecordIssueOpsHeartbeatWithRequest(stateRoot, hb)
	if err != nil || updated.ExecutionHandoff.LastHeartbeatAt == "" {
		t.Fatalf("heartbeat failed: %#v err=%v", updated.ExecutionHandoff, err)
	}
	hb.SessionID = "stale"
	if _, err := RecordIssueOpsHeartbeatWithRequest(stateRoot, hb); err == nil {
		t.Fatal("stale worker heartbeat must fail")
	}
}

func TestHandoffFinishSubmitAcceptLifecycle(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	finish := handoffFinishRequest(claim, claimed)
	submitted, err := FinishIssueOpsHandoff(stateRoot, finish)
	if err != nil {
		t.Fatal(err)
	}
	if submitted.ExecutionHandoff.State != handoff.StateSubmitted {
		t.Fatalf("expected submitted: %#v", submitted.ExecutionHandoff)
	}
	accepted, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.ExecutionHandoff.State != handoff.StateClosed || accepted.ExecutionHandoff.ClosedDisposition != handoff.DispositionAccepted {
		t.Fatalf("expected accepted close: %#v", accepted.ExecutionHandoff)
	}
}

func TestHandoffFinishFailureClosesWorkerFailed(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	failed, err := FinishIssueOpsHandoff(stateRoot, IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeFailed,
		Verification: []string{"go test failed"}, CleanupReceipts: []string{"temp state removed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if failed.ExecutionHandoff.ClosedDisposition != handoff.DispositionWorkerFailed {
		t.Fatalf("unexpected failure close: %#v", failed.ExecutionHandoff)
	}
}

func TestHandoffFinishAndAcceptIdempotency(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	claimed, _ := ClaimIssueOpsHandoff(stateRoot, claim)
	finish := handoffFinishRequest(claim, claimed)
	first, err := FinishIssueOpsHandoff(stateRoot, finish)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FinishIssueOpsHandoff(stateRoot, finish)
	if err != nil || !reflect.DeepEqual(first.ExecutionHandoff.Result, second.ExecutionHandoff.Result) {
		t.Fatalf("finish not idempotent: err=%v", err)
	}
	accept := IssueOpsHandoffAcceptRequest{ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead}
	if _, err := AcceptIssueOpsHandoff(stateRoot, accept); err != nil {
		t.Fatal(err)
	}
	if _, err := AcceptIssueOpsHandoff(stateRoot, accept); err != nil {
		t.Fatalf("accept not idempotent: %v", err)
	}
	finish.FinalHead = "conflict"
	if _, err := FinishIssueOpsHandoff(stateRoot, finish); err == nil {
		t.Fatal("conflicting finish must fail")
	}
}

func TestHandoffResumeIsReadOnly(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	_, source, _ := dispatchedHandoffRecordAt(t, stateRoot)
	before, _ := json.Marshal(source)
	result := IssueOpsResume(source.Repo, source.ID)
	if !result.OK || result.ExecutionHandoff == nil || result.ExecutionHandoff.State != handoff.StateDispatched {
		t.Fatalf("unexpected resume projection: %#v", result)
	}
	afterRecord, err := ReadIssueOps(stateRoot, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(afterRecord)
	if string(before) != string(after) {
		t.Fatal("resume mutated durable state")
	}
}

func TestInlineHeartbeatAndResumeRemainCompatible(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "16-inline"})
	if err != nil {
		t.Fatal(err)
	}
	if updated, err := RecordIssueOpsHeartbeat(IssueOpsStateRoot(), record.ID); err != nil || updated.LastHeartbeatAt == "" {
		t.Fatalf("inline heartbeat changed: %#v err=%v", updated, err)
	}
	if result := IssueOpsResume(repo, record.ID); !result.OK {
		t.Fatalf("inline resume changed: %#v", result)
	}
}

func dispatchedHandoffRecord(t *testing.T) (string, IssueOpsRecord, *dispatchOrcaFake) {
	t.Helper()
	return dispatchedHandoffRecordAt(t, t.TempDir())
}

func dispatchedHandoffRecordAt(t *testing.T, stateRoot string) (string, IssueOpsRecord, *dispatchOrcaFake) {
	t.Helper()
	originalState, record := handoffDispatchRecord(t)
	if stateRoot != originalState {
		if _, err := WriteIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
	}
	client := handoffDispatchFake()
	dispatched, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReadIssueOps(stateRoot, dispatched.ID)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, result, client
}

func handoffClaimRequest(record IssueOpsRecord) IssueOpsHandoffClaimRequest {
	return IssueOpsHandoffClaimRequest{
		ID: record.ID, Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch,
		ContextSHA256: record.ExecutionHandoff.ContextSHA256, Host: "codex", SessionID: "session-1", AgentID: "codex-worker",
		CWD: record.WorktreePath, OrcaWorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
	}
}

func handoffFinishRequest(claim IssueOpsHandoffClaimRequest, record IssueOpsRecord) IssueOpsHandoffFinishRequest {
	return IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeCompleted,
		FinalHead: "head-1", ChangedFiles: []string{"internal/demo.go"}, TuringReportPath: ".agent-harness/research/report.md",
		Verification: []string{"go test ./...: pass"}, CleanupReceipts: []string{"temporary state removed"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}
}
