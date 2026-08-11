package issueopsrecord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
)

const defaultSlowSpanThreshold = 100 * time.Millisecond

type SpanObservation struct {
	Event       string `json:"event"`
	GeneratedAt string `json:"generated_at"`
	Operation   string `json:"operation"`
	Outcome     string `json:"outcome"`
	Contended   bool   `json:"contended"`
	WaitMS      int64  `json:"wait_ms"`
	HoldMS      int64  `json:"hold_ms"`
}

type Observer interface {
	Observe(SpanObservation)
}

type ObserverFunc func(SpanObservation)

func (observe ObserverFunc) Observe(observation SpanObservation) {
	observe(observation)
}

type jsonLineObserver struct {
	writer        io.Writer
	slowThreshold time.Duration
	mutex         sync.Mutex
}

func NewJSONLineObserver(writer io.Writer, slowThreshold time.Duration) Observer {
	if slowThreshold <= 0 {
		slowThreshold = defaultSlowSpanThreshold
	}
	return &jsonLineObserver{writer: writer, slowThreshold: slowThreshold}
}

func (observer *jsonLineObserver) Observe(observation SpanObservation) {
	if observer == nil || observer.writer == nil {
		return
	}
	thresholdMS := observer.slowThreshold.Milliseconds()
	if observation.Outcome == sqlstore.SpanOutcomeSuccess &&
		!observation.Contended &&
		observation.WaitMS < thresholdMS &&
		observation.HoldMS < thresholdMS {
		return
	}
	observation.Event = "issueops_record_span"
	if strings.TrimSpace(observation.GeneratedAt) == "" {
		observation.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(observation)
	if err != nil {
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if _, err := observer.writer.Write(append(line, '\n')); err != nil && observer.writer != os.Stderr {
		_, _ = fmt.Fprintf(os.Stderr, "issueops span observability write failed: %v\n", err)
	}
}

func (store Store) observe(ctx context.Context, operation string) context.Context {
	if store.Observer == nil {
		return ctx
	}
	scope := strings.TrimSpace(store.Scope)
	if scope == "" {
		scope = "issueops"
	}
	return sqlstore.WithSpanObserver(ctx, func(observation sqlstore.SpanObservation) {
		store.Observer.Observe(SpanObservation{
			Operation: scope + "." + operation,
			Outcome:   observation.Outcome,
			Contended: observation.Contended,
			WaitMS:    max(0, observation.Wait.Milliseconds()),
			HoldMS:    max(0, observation.Hold.Milliseconds()),
		})
	})
}
