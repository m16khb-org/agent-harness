package sqlstore

import (
	"context"
	"errors"
	"testing"
)

func TestWithSpanReportsSuccessAndFailureAfterReleasingLock(t *testing.T) {
	database, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var observations []SpanObservation
	ctx := WithSpanObserver(context.Background(), func(observation SpanObservation) {
		observations = append(observations, observation)
	})

	if err := database.WithSpan(ctx, func(context.Context) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("mutation failed")
	if err := database.WithSpan(ctx, func(context.Context) error {
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("callback error = %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("observations=%d want=2", len(observations))
	}
	if observations[0].Outcome != SpanOutcomeSuccess ||
		observations[0].Wait < 0 ||
		observations[0].Hold < 0 {
		t.Fatalf("success observation = %+v", observations[0])
	}
	if observations[1].Outcome != SpanOutcomeError {
		t.Fatalf("failure observation = %+v", observations[1])
	}

	if err := database.WithSpan(context.Background(), func(context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("observer ran before releasing span lock: %v", err)
	}
}
