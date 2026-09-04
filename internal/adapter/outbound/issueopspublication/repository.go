package issueopspublication

import (
	"context"
	"fmt"

	application "issueops/internal/application/issueopspublication"
	contract "issueops/internal/contract/issueopspublication"
)

type Effects interface {
	PreviewCreate(context.Context, contract.CreateCommand) (EffectState, error)
	BeginCreate(context.Context, contract.CreateCommand) (EffectState, error)
	LoadIntent(context.Context, string) (EffectState, error)
	MarkRetry(context.Context, EffectState) (EffectState, error)
	RecordFailure(context.Context, EffectState, contract.InvocationState, string, error) error
	Complete(context.Context, EffectState, string, bool) (EffectState, error)
	CompleteNotInvoked(context.Context, EffectState, error) (EffectState, error)
	Latest(context.Context, string) (EffectState, error)
}

type EffectState struct {
	RecordID    string
	RecordRaw   []byte
	IntentRaw   []byte
	OperationID string
	Generation  uint64
	Provider    string
	Kind        string
	Request     contract.ProviderCreateRequest
	Eligibility contract.CreateEligibility

	InvocationState contract.InvocationState
	RetryCount      int
	KnownURL        string
}

type Repository struct{ effects Effects }

func NewRepository(effects Effects) *Repository {
	return &Repository{effects: effects}
}

func (r *Repository) PreviewCreate(ctx context.Context, command contract.CreateCommand) (contract.PreparedCreate, error) {
	if r == nil || r.effects == nil {
		return contract.PreparedCreate{}, fmt.Errorf("publication persistence bridge is required")
	}
	state, err := r.effects.PreviewCreate(ctx, command.Clone())
	return preparedCreate(state), err
}

func (r *Repository) BeginCreate(ctx context.Context, command contract.CreateCommand) (contract.Intent, error) {
	if r == nil || r.effects == nil {
		return contract.Intent{}, fmt.Errorf("publication persistence bridge is required")
	}
	state, err := r.effects.BeginCreate(ctx, command.Clone())
	return intentState(state), err
}

func (r *Repository) LoadIntent(ctx context.Context, id string) (contract.Intent, error) {
	if r == nil || r.effects == nil {
		return contract.Intent{}, fmt.Errorf("publication persistence bridge is required")
	}
	state, err := r.effects.LoadIntent(ctx, id)
	return intentState(state), err
}

func (r *Repository) MarkRetry(ctx context.Context, intent contract.Intent) (contract.Intent, error) {
	if r == nil || r.effects == nil {
		return contract.Intent{}, fmt.Errorf("publication persistence bridge is required")
	}
	state, err := r.effects.MarkRetry(ctx, effectState(intent))
	return intentState(state), err
}

func (r *Repository) RecordFailure(ctx context.Context, intent contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
	if r == nil || r.effects == nil {
		return fmt.Errorf("publication persistence bridge is required")
	}
	return r.effects.RecordFailure(ctx, effectState(intent), invocation, knownURL, cause)
}

func (r *Repository) Complete(ctx context.Context, intent contract.Intent, url string, enforceOriginalGeneration bool) (contract.RecordSnapshot, error) {
	if r == nil || r.effects == nil {
		return contract.RecordSnapshot{}, fmt.Errorf("publication persistence bridge is required")
	}
	state, err := r.effects.Complete(ctx, effectState(intent), url, enforceOriginalGeneration)
	return recordSnapshot(state), err
}

func (r *Repository) CompleteNotInvoked(ctx context.Context, intent contract.Intent, cause error) (contract.RecordSnapshot, error) {
	if r == nil || r.effects == nil {
		return contract.RecordSnapshot{}, fmt.Errorf("publication persistence bridge is required")
	}
	state, err := r.effects.CompleteNotInvoked(ctx, effectState(intent), cause)
	return recordSnapshot(state), err
}

func (r *Repository) Latest(ctx context.Context, id string) (contract.RecordSnapshot, error) {
	if r == nil || r.effects == nil {
		return contract.RecordSnapshot{}, fmt.Errorf("publication persistence bridge is required")
	}
	state, err := r.effects.Latest(ctx, id)
	return recordSnapshot(state), err
}

func preparedCreate(state EffectState) contract.PreparedCreate {
	return contract.PreparedCreate{Request: state.Request.Clone(), Eligibility: state.Eligibility}
}

func intentState(state EffectState) contract.Intent {
	return contract.Intent{
		Record:      contract.RecordSnapshot{ID: state.RecordID, Raw: append([]byte(nil), state.RecordRaw...)},
		OperationID: state.OperationID, Generation: state.Generation, Provider: state.Provider, Kind: state.Kind,
		Request: state.Request.Clone(), InvocationState: state.InvocationState, RetryCount: state.RetryCount,
		KnownURL: state.KnownURL, Eligibility: state.Eligibility, Raw: append([]byte(nil), state.IntentRaw...),
	}
}

func effectState(intent contract.Intent) EffectState {
	return EffectState{
		RecordID: intent.Record.ID, RecordRaw: append([]byte(nil), intent.Record.Raw...), IntentRaw: append([]byte(nil), intent.Raw...),
		OperationID: intent.OperationID, Generation: intent.Generation, Provider: intent.Provider, Kind: intent.Kind,
		Request: intent.Request.Clone(), InvocationState: intent.InvocationState, RetryCount: intent.RetryCount,
		KnownURL: intent.KnownURL, Eligibility: intent.Eligibility,
	}
}

func recordSnapshot(state EffectState) contract.RecordSnapshot {
	return contract.RecordSnapshot{ID: state.RecordID, Raw: append([]byte(nil), state.RecordRaw...)}
}

var _ application.Repository = (*Repository)(nil)
