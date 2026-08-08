package issueopslease

import (
	"context"
	"errors"
	"strings"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

type reconcileRepositoryFake struct {
	state        ReconcileIntentState
	canonicalErr error
	markCalls    int
	failureCalls int
	applyCalls   int
	failureState string
	latest       leasecontract.Record
	cleared      bool
	clearedCause error
	clearErr     error
}

func (f *reconcileRepositoryFake) Canonicalize(context.Context, string) (ReconcileIntentState, error) {
	return f.state, f.canonicalErr
}

func (f *reconcileRepositoryFake) MarkInvoking(_ context.Context, state ReconcileIntentState) (ReconcileIntentState, error) {
	f.markCalls++
	state.InvocationAttempts++
	state.InvocationState = "unknown"
	return state, nil
}

func (f *reconcileRepositoryFake) RecordFailure(_ context.Context, _ ReconcileIntentState, invocation string, _ error) error {
	f.failureCalls++
	f.failureState = invocation
	return nil
}

func (f *reconcileRepositoryFake) ApplyReceipt(_ context.Context, state ReconcileIntentState, _ leasecontract.ReconcileStageReceipt) (ReconcileProgress, error) {
	f.applyCalls++
	next := map[string]string{
		"worktree_create": "terminal_create",
		"terminal_create": "run_create",
		"run_create":      "run_bind",
		"run_bind":        "task_create",
		"task_create":     "dispatch",
	}[state.Stage]
	return ReconcileProgress{Record: state.Progress.Record, Pending: state.Stage != "dispatch", NextStage: next}, nil
}

func (f *reconcileRepositoryFake) Latest(context.Context, string) (leasecontract.Record, error) {
	if f.latest.ID != "" {
		return f.latest, nil
	}
	return f.state.Progress.Record, nil
}

type reconcileStageExecutorFake struct {
	inventory    leasecontract.ReconcileStageInventory
	attempted    bool
	inspectErr   error
	invokeErr    error
	failureState string
	inspectCalls int
	invokeCalls  int
}

func (f *reconcileStageExecutorFake) Inspect(context.Context, ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
	f.inspectCalls++
	return f.inventory, f.attempted, f.inspectErr
}

func (f *reconcileStageExecutorFake) Invoke(context.Context, ReconcileIntentState) (leasecontract.ReconcileStageReceipt, string, error) {
	f.invokeCalls++
	return leasecontract.ReconcileStageReceipt{TaskID: "task-1"}, f.failureState, f.invokeErr
}

func TestReconcileServiceAdoptsOneCandidateAndAdvancesOneStage(t *testing.T) {
	repository := reconcileRepositoryFixture("run_bind", "unknown", 1)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{Candidates: []leasecontract.ReconcileStageReceipt{{RunID: "run-1", RunBound: true}}}}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Reconciled || !result.ExternalStateInspected || result.Code != "orca_reconcile_advanced_task_create" {
		t.Fatalf("result = %#v", result)
	}
	if stages.inspectCalls != 1 || stages.invokeCalls != 0 || repository.applyCalls != 1 {
		t.Fatalf("calls inspect=%d invoke=%d apply=%d", stages.inspectCalls, stages.invokeCalls, repository.applyCalls)
	}
}

func TestReconcileServiceInvokesOnlyProvenSafeZero(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 1)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != "orca_reconcile_advanced_dispatch" || repository.markCalls != 1 || stages.invokeCalls != 1 || repository.applyCalls != 1 {
		t.Fatalf("result=%#v mark=%d invoke=%d apply=%d", result, repository.markCalls, stages.invokeCalls, repository.applyCalls)
	}
}

// TestReconcileServiceClearsAnUnknownOutcomeWithNoResource는 #280을 고정한다.
//
// 예전에는 이 상태를 preserve해서 intent가 영원히 수렴하지 않았다. 외부
// 인벤토리가 authoritative zero를 돌려줬다면 그 mutation은 자원을 남기지
// 않았고, 남은 intent는 사실이 아니라 기록이다. 재시도가 아니라 정리다.
func TestReconcileServiceClearsAnUnknownOutcomeWithNoResource(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "unknown", 1)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err != nil {
		t.Fatalf("자원이 없음이 확정된 intent는 수렴해야 한다: %v", err)
	}
	if result.Code != "orca_reconcile_cleared" || !result.Reconciled || !repository.cleared {
		t.Fatalf("result=%#v cleared=%v", result, repository.cleared)
	}
	// 재시도하지 않았음을 고정한다 — 그게 이 경로의 안전 근거다.
	if repository.markCalls != 0 || stages.invokeCalls != 0 || repository.applyCalls != 0 {
		t.Fatalf("mutation을 재시도하면 안 된다: mark=%d invoke=%d apply=%d",
			repository.markCalls, stages.invokeCalls, repository.applyCalls)
	}
	// 실패가 아니라 종결이므로 실패 기록을 남기지 않는다.
	if repository.failureCalls != 0 {
		t.Fatalf("정리는 실패 기록을 남기지 않아야 한다: %d", repository.failureCalls)
	}
	if repository.clearedCause == nil || !strings.Contains(repository.clearedCause.Error(), "left no external resource") {
		t.Fatalf("제거 사유가 record에 남아야 한다: %v", repository.clearedCause)
	}
}

// TestReconcileServiceStillPreservesANonAuthoritativeZero는 완화가 관측
// 실패까지 삼키지 않음을 고정한다. authoritative하지 않은 zero는 자원 부재의
// 증거가 아니다.
func TestReconcileServiceStillPreservesANonAuthoritativeZero(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "unknown", 1)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: false}}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || !strings.Contains(err.Error(), "non-authoritative zero") {
		t.Fatalf("비authoritative zero는 계속 보존돼야 한다: %v", err)
	}
	if result.Code != "orca_reconcile_ambiguous" || repository.cleared || repository.failureCalls != 1 {
		t.Fatalf("result=%#v cleared=%v failures=%d", result, repository.cleared, repository.failureCalls)
	}
}

// TestReconcileServiceSurfacesAFailedClear는 제거 자체가 실패하면 그것을
// 삼키지 않음을 고정한다.
func TestReconcileServiceSurfacesAFailedClear(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "unknown", 1)
	repository.clearErr = errors.New("state moved under the clear")
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || !strings.Contains(err.Error(), "state moved under the clear") {
		t.Fatalf("제거 실패는 표면화돼야 한다: %v", err)
	}
	if result.Code != "orca_reconcile_ambiguous" || repository.failureCalls != 1 {
		t.Fatalf("result=%#v failures=%d", result, repository.failureCalls)
	}
}

func TestReconcileServiceDisclosesOnlyActualInspectionAttempt(t *testing.T) {
	for _, test := range []struct {
		name      string
		attempted bool
	}{
		{name: "local validation", attempted: false},
		{name: "transport", attempted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
			stages := &reconcileStageExecutorFake{attempted: test.attempted, inspectErr: errors.New("inspect failed")}
			result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
			if err == nil || result.ExternalStateInspected != test.attempted {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			wantFailures := 0
			if test.attempted {
				wantFailures = 1
			}
			if repository.failureCalls != wantFailures {
				t.Fatalf("failure calls = %d, want %d", repository.failureCalls, wantFailures)
			}
		})
	}
}

func TestReconcileServiceCanonicalizationFailsBeforeInspection(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
	repository.canonicalErr = errors.New("unsafe marker")
	stages := &reconcileStageExecutorFake{}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || result.Code != "orca_intent_invalid" || result.ExternalStateInspected || stages.inspectCalls != 0 {
		t.Fatalf("result=%#v inspect=%d err=%v", result, stages.inspectCalls, err)
	}
}

func reconcileRepositoryFixture(stage, invocation string, attempts int) *reconcileRepositoryFake {
	record := leasecontract.Record{OK: true, ID: "io-1", Execution: &leasecontract.Execution{Pending: &leasecontract.ExternalIntent{Kind: "owner_launch"}}}
	return &reconcileRepositoryFake{state: ReconcileIntentState{Progress: ReconcileProgress{Record: record, Pending: true, NextStage: stage}, OperationID: "op-1", Stage: stage, InvocationState: invocation, InvocationAttempts: attempts}}
}

func (f *reconcileRepositoryFake) ClearIntent(_ context.Context, state ReconcileIntentState, cause error) (ReconcileProgress, error) {
	f.clearedCause = cause
	f.cleared = true
	return ReconcileProgress{Record: state.Progress.Record, Pending: false}, f.clearErr
}
