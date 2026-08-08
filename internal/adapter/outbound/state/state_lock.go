package state

import (
	"context"

	statecontract "agent-harness/internal/contract/state"
)

func StateUpdate(key string, transform func(statecontract.RecordEnvelope) (statecontract.RecordEnvelope, error)) (statecontract.StateResult, error) {
	return service().Update(key, transform)
}

func WithKeyLock(ctx context.Context, dir, key string, fn func(context.Context) error) error {
	return service().WithKeyLock(ctx, dir, key, fn)
}

func withStateLock(ctx context.Context, dir, key string, fn func(context.Context) error) error {
	return service().WithKeyLock(ctx, dir, key, fn)
}
