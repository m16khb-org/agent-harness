package issueopslease

import (
	"bytes"
	"context"
	"errors"
	"testing"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
)

type reconcileEffectsFake struct {
	state ReconcileEffectState
}

func (f *reconcileEffectsFake) Canonicalize(context.Context, string) (ReconcileEffectState, error) {
	return f.state, nil
}
func (f *reconcileEffectsFake) MarkInvoking(context.Context, ReconcileEffectState) (ReconcileEffectState, error) {
	return f.state, nil
}
func (f *reconcileEffectsFake) RecordFailure(context.Context, ReconcileEffectState, string, error) error {
	return nil
}
func (f *reconcileEffectsFake) ApplyReceipt(context.Context, ReconcileEffectState, leasecontract.ReconcileStageReceipt) (ReconcileEffectState, error) {
	return f.state, nil
}
func (f *reconcileEffectsFake) Latest(context.Context, string) (leasecontract.Record, error) {
	return f.state.Record, nil
}

func TestReconcileRepositoryPreservesRawCASState(t *testing.T) {
	effects := &reconcileEffectsFake{state: ReconcileEffectState{
		Record:    leasecontract.Record{ID: "io-1", Execution: &leasecontract.Execution{}},
		RecordRaw: []byte("record-raw"), IntentRaw: []byte("intent-raw"), OperationID: "op-1",
		Stage: "run_bind", InvocationState: "unknown", InvocationAttempts: 1, Pending: true,
	}}
	state, err := NewReconcileRepository(effects).Canonicalize(context.Background(), "io-1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(state.RecordRaw, effects.state.RecordRaw) || !bytes.Equal(state.IntentRaw, effects.state.IntentRaw) {
		t.Fatalf("state = %#v", state)
	}
}

func TestReconcileStageExecutorPreservesAttemptDisclosure(t *testing.T) {
	wantErr := errors.New("transport")
	executor := NewReconcileStageExecutor(
		func(context.Context, leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
			return leasecontract.ReconcileStageInventory{}, true, wantErr
		},
		nil,
	)
	_, attempted, err := executor.Inspect(context.Background(), leaseapp.ReconcileIntentState{})
	if !attempted || !errors.Is(err, wantErr) {
		t.Fatalf("attempted=%t err=%v", attempted, err)
	}
}
