package issueopsrouting

import (
	"context"
	"testing"
	"time"

	issueopsroutingapplication "issueops/internal/application/issueopsrouting"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
)

type fakeRoutingStore struct {
	readCalls   int
	updateCalls int
	record      issueopsroutingcontract.Record
}

func (store *fakeRoutingStore) Read(context.Context, string, string) (issueopsroutingcontract.Record, error) {
	store.readCalls++
	return store.record, nil
}

func (store *fakeRoutingStore) Update(
	_ context.Context,
	_ string,
	_ string,
	mutate func(issueopsroutingcontract.Record) (issueopsroutingcontract.Record, bool, error),
) (issueopsroutingcontract.Record, error) {
	store.updateCalls++
	next, _, err := mutate(issueopsroutingcontract.Record{ID: "io-3"})
	if err != nil {
		return issueopsroutingcontract.Record{OK: false}, err
	}
	store.record = next
	return next, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type identityPaths struct{}

func (identityPaths) Same(a, b string) bool { return a == b }

func TestRoutingHandlersDelegateRecordAndScore(t *testing.T) {
	store := &fakeRoutingStore{}
	handlers := NewHandlers(issueopsroutingapplication.NewService(
		store,
		fixedClock{now: time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)},
		identityPaths{},
	))

	actor := issueopsroutingcontract.Actor{Host: "codex"}
	record, err := handlers.Record("/state", "io-3", "plan", "verified-execution", actor)
	if err != nil {
		t.Fatalf("record failed: %v", err)
	}
	if store.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", store.updateCalls)
	}
	if record.ID != "io-3" {
		t.Fatalf("record id drift: %q", record.ID)
	}

	store.record.RoutingTrace = []issueopsroutingcontract.Entry{{}, {}}
	_, traceDepth, err := handlers.Score("/state", "io-3", nil)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}
	if store.readCalls != 1 {
		t.Fatalf("read calls = %d, want 1", store.readCalls)
	}
	if traceDepth != 2 {
		t.Fatalf("trace depth = %d, want 2", traceDepth)
	}
}

func TestScorePropagatesReadState(t *testing.T) {
	store := &fakeRoutingStore{}
	handlers := NewHandlers(issueopsroutingapplication.NewService(
		store,
		fixedClock{},
		identityPaths{},
	))

	_, depth, err := handlers.Score("/state", "io-empty", nil)
	if err != nil {
		t.Fatalf("score on empty record failed: %v", err)
	}
	if depth != 0 {
		t.Fatalf("depth = %d, want 0 for empty routing trace", depth)
	}
}

func TestNilServiceFailsClosed(t *testing.T) {
	handlers := NewHandlers(nil)
	if _, err := handlers.Record("/state", "io-4", "plan", "verified-execution", issueopsroutingcontract.Actor{}); err == nil {
		t.Fatal("nil service must fail closed on record")
	}
	if _, _, err := handlers.Score("/state", "io-4", nil); err == nil {
		t.Fatal("nil service must fail closed on score")
	}
}
