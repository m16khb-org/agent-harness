package issueopslease

import (
	"context"
	"fmt"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

type ResumeEffects interface {
	Begin(context.Context, leasecontract.Record, []byte, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, string) (ResumeEffectState, error)
	Read(context.Context, string, string) (ResumeEffectState, error)
	MarkInvoking(context.Context, ResumeEffectState) (ResumeEffectState, error)
	RecordFailure(context.Context, ResumeEffectState, string, error) error
	ApplyReceipt(context.Context, ResumeEffectState, leasecontract.ResumeStageReceipt) (ResumeEffectState, error)
}

type ResumeEffectState struct {
	Record             leasecontract.Record
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              string
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

type ResumeRepository struct {
	store   port.TransactionalRecordStore
	effects ResumeEffects
}

func NewResumeRepository(store port.TransactionalRecordStore, effects ResumeEffects) *ResumeRepository {
	return &ResumeRepository{store: store, effects: effects}
}

func (r *ResumeRepository) LoadSnapshot(_ context.Context, id string, generation uint64) (leaseapp.ResumeSnapshot, error) {
	if r == nil || r.store == nil {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	data, ok, err := r.store.Get(recordBucket, id)
	if err != nil {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := decodeLeaseRecord(id, data)
	if err != nil {
		return leaseapp.ResumeSnapshot{}, err
	}
	if record.Execution == nil {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	if generation == 0 || record.Execution.Lease.Generation != generation {
		return leaseapp.ResumeSnapshot{}, fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, generation)
	}
	return leaseapp.ResumeSnapshot{Record: toApplicationRecord(record), Raw: append([]byte(nil), data...)}, nil
}

func (r *ResumeRepository) BeginIntent(ctx context.Context, snapshot leaseapp.ResumeSnapshot, artifacts leasecontract.ResumeArtifacts, plan leasedomain.ResumePlan, operationID string) (leaseapp.ResumeProgress, error) {
	if r == nil || r.effects == nil {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume persistence bridge is required"))
	}
	state, err := r.effects.Begin(ctx, snapshot.Record.Stable, snapshot.Raw, artifacts, plan, operationID)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	return resumeProgress(state), nil
}

func (r *ResumeRepository) LoadIntent(ctx context.Context, progress leaseapp.ResumeProgress) (leaseapp.ResumeIntentState, error) {
	if r == nil || r.effects == nil || progress.Execution.Pending == nil {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume pending intent is required"))
	}
	state, err := r.effects.Read(ctx, progress.Record.ID, progress.Execution.Pending.OperationID)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	return resumeIntentState(state), nil
}

func (r *ResumeRepository) MarkInvoking(ctx context.Context, intent leaseapp.ResumeIntentState) (leaseapp.ResumeIntentState, error) {
	if r == nil || r.effects == nil {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume persistence bridge is required"))
	}
	state, err := r.effects.MarkInvoking(ctx, resumeEffectState(intent))
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	return resumeIntentState(state), nil
}

func (r *ResumeRepository) RecordFailure(ctx context.Context, intent leaseapp.ResumeIntentState, invocation string, cause error) error {
	if r == nil || r.effects == nil {
		return leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume persistence bridge is required"))
	}
	return r.effects.RecordFailure(ctx, resumeEffectState(intent), invocation, cause)
}

func (r *ResumeRepository) ApplyReceipt(ctx context.Context, intent leaseapp.ResumeIntentState, receipt leasecontract.ResumeStageReceipt) (leaseapp.ResumeProgress, error) {
	if r == nil || r.effects == nil {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume persistence bridge is required"))
	}
	state, err := r.effects.ApplyReceipt(ctx, resumeEffectState(intent), receipt)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	return resumeProgress(state), nil
}

func resumeProgress(state ResumeEffectState) leaseapp.ResumeProgress {
	return leaseapp.ResumeProgress{Record: toApplicationRecord(state.Record), Execution: *state.Record.Execution, Pending: state.Pending}
}

func resumeIntentState(state ResumeEffectState) leaseapp.ResumeIntentState {
	return leaseapp.ResumeIntentState{Progress: resumeProgress(state), OperationID: state.OperationID, Stage: state.Stage, InvocationState: state.InvocationState, InvocationAttempts: state.InvocationAttempts, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...)}
}

func resumeEffectState(intent leaseapp.ResumeIntentState) ResumeEffectState {
	return ResumeEffectState{Record: intent.Progress.Record.Stable, RecordRaw: append([]byte(nil), intent.RecordRaw...), IntentRaw: append([]byte(nil), intent.IntentRaw...), OperationID: intent.OperationID, Stage: intent.Stage, InvocationState: intent.InvocationState, InvocationAttempts: intent.InvocationAttempts, Pending: intent.Progress.Pending}
}
