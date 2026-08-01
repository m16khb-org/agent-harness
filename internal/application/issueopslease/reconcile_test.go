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

func TestReconcileServicePreservesUnknownCreateOutcome(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "unknown", 1)
	stages := &reconcileStageExecutorFake{attempted: true, inventory: leasecontract.ReconcileStageInventory{AuthoritativeZero: true}}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || !strings.Contains(err.Error(), "absence was not proven") {
		t.Fatalf("err = %v", err)
	}
	if result.Code != "orca_reconcile_ambiguous" || !result.ExternalStateInspected || repository.failureCalls != 1 || repository.markCalls != 0 || stages.invokeCalls != 0 || repository.applyCalls != 0 {
		t.Fatalf("result=%#v failures=%d mark=%d invoke=%d apply=%d", result, repository.failureCalls, repository.markCalls, stages.invokeCalls, repository.applyCalls)
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

func TestReconcileServiceReportsMigrationOnAmbiguousOutcome(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
	repository.state.Migrated = true
	stages := &reconcileStageExecutorFake{attempted: true, inspectErr: errors.New("transport")}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || !result.IntentMigrated || result.Code != "orca_reconcile_ambiguous" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestReconcileServiceCanonicalizationFailsBeforeInspection(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
	repository.canonicalErr = errors.New("unsafe marker")
	stages := &reconcileStageExecutorFake{}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || result.Code != "legacy_intent_upgrade_unsafe" || result.ExternalStateInspected || stages.inspectCalls != 0 {
		t.Fatalf("result=%#v inspect=%d err=%v", result, stages.inspectCalls, err)
	}
}

func TestReconcileServicePreservesPartialMigrationDisclosureOnCanonicalizationFailure(t *testing.T) {
	repository := reconcileRepositoryFixture("task_create", "not_invoked_proven", 0)
	repository.state.Migrated = true
	repository.canonicalErr = errors.New("snapshot changed after migration")
	stages := &reconcileStageExecutorFake{}
	result, err := NewReconcileService(repository, stages).Reconcile(context.Background(), ReconcileRequest{ID: "io-1"})
	if err == nil || !result.IntentMigrated || result.Record.ID != "io-1" || result.Code != "legacy_intent_upgrade_unsafe" || stages.inspectCalls != 0 {
		t.Fatalf("result=%#v inspect=%d err=%v", result, stages.inspectCalls, err)
	}
}

func reconcileRepositoryFixture(stage, invocation string, attempts int) *reconcileRepositoryFake {
	record := leasecontract.Record{OK: true, ID: "io-1", Execution: &leasecontract.Execution{Pending: &leasecontract.ExternalIntent{Kind: "owner_launch"}}}
	return &reconcileRepositoryFake{state: ReconcileIntentState{Progress: ReconcileProgress{Record: record, Pending: true, NextStage: stage}, OperationID: "op-1", Stage: stage, InvocationState: invocation, InvocationAttempts: attempts}}
}
