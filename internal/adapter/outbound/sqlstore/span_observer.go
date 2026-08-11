package sqlstore

import (
	"context"
	"time"
)

const (
	SpanOutcomeSuccess  = "success"
	SpanOutcomeError    = "error"
	SpanOutcomeCanceled = "canceled"
	SpanOutcomeNested   = "nested"
)

type SpanObservation struct {
	Outcome   string
	Contended bool
	Wait      time.Duration
	Hold      time.Duration
}

type SpanObserver func(SpanObservation)

type spanObserverKey struct{}

func WithSpanObserver(ctx context.Context, observer SpanObserver) context.Context {
	if ctx == nil || observer == nil {
		return ctx
	}
	return context.WithValue(ctx, spanObserverKey{}, observer)
}

func spanObserver(ctx context.Context) SpanObserver {
	if ctx == nil {
		return nil
	}
	observer, _ := ctx.Value(spanObserverKey{}).(SpanObserver)
	return observer
}
