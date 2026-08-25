package issueopsretention

import (
	"context"
	"errors"
	"testing"
	"time"

	issueopsretentionapplication "agent-harness/internal/application/issueopsretention"
	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
)

type fakeRetentionStore struct {
	listIDsCalls int
	ids          []string
	err          error
}

func (store *fakeRetentionStore) ListIDs(context.Context, string) ([]string, error) {
	store.listIDsCalls++
	return store.ids, store.err
}

func (store *fakeRetentionStore) ReadUnchecked(
	context.Context, string, string,
) (issueopsretentioncontract.Record, error) {
	return issueopsretentioncontract.Record{}, errors.New("not expected in this test")
}

func (store *fakeRetentionStore) DeleteIfUnchanged(
	context.Context, string, string, issueopsretentioncontract.Record,
) error {
	return errors.New("not expected in this test")
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func newPruneHandler(store *fakeRetentionStore) func(string, time.Duration, bool) (issueopsretentioncontract.Result, error) {
	return NewPruneHandler(issueopsretentionapplication.NewService(
		store,
		fixedClock{now: time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)},
	))
}

func TestPruneHandlerReportsDryRunByDefault(t *testing.T) {
	store := &fakeRetentionStore{}
	handler := newPruneHandler(store)

	result, err := handler("/state", 24*time.Hour, false)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if !result.DryRun {
		t.Fatal("confirm=false must produce a dry run")
	}
	if result.StateRoot != "/state" || result.MaxAge != (24*time.Hour).String() {
		t.Fatalf("arguments did not pass through: %+v", result)
	}
	if store.listIDsCalls != 1 {
		t.Fatalf("list ids calls = %d, want 1", store.listIDsCalls)
	}
}

func TestPruneHandlerSwitchesToConfirmedMode(t *testing.T) {
	store := &fakeRetentionStore{}
	handler := newPruneHandler(store)

	result, err := handler("/state", time.Hour, true)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if result.DryRun {
		t.Fatal("confirm=true must not be a dry run")
	}
}

func TestPruneHandlerRejectsNonPositiveMaxAge(t *testing.T) {
	store := &fakeRetentionStore{}
	handler := newPruneHandler(store)

	if _, err := handler("/state", 0, false); err == nil {
		t.Fatal("non-positive max age must fail")
	}
	if store.listIDsCalls != 0 {
		t.Fatal("repository must not be touched when max age is invalid")
	}
}

func TestPruneHandlerPropagatesListError(t *testing.T) {
	store := &fakeRetentionStore{err: errors.New("lock held")}
	handler := newPruneHandler(store)

	if _, err := handler("/state", time.Hour, false); err == nil {
		t.Fatal("list ids failure must surface through handler")
	}
}
