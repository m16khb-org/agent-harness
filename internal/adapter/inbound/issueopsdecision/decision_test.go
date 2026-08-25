package issueopsdecision

import (
	"context"
	"strings"
	"testing"
	"time"

	issueopsdecisionapplication "agent-harness/internal/application/issueopsdecision"
	issueopsdecisioncontract "agent-harness/internal/contract/issueopsdecision"
)

type fakeRepository struct {
	updateCalls int
	stateRoot   string
	id          string
	err         error
}

func (repo *fakeRepository) Update(
	_ context.Context,
	stateRoot string,
	id string,
	mutate func(issueopsdecisioncontract.Record) (issueopsdecisioncontract.Record, error),
) (issueopsdecisioncontract.Record, error) {
	repo.updateCalls++
	repo.stateRoot = stateRoot
	repo.id = id
	if repo.err != nil {
		return issueopsdecisioncontract.Record{OK: false, ID: id}, repo.err
	}
	return mutate(issueopsdecisioncontract.Record{ID: id})
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type identityPaths struct{}

func (identityPaths) Same(a, b string) bool { return a == b }

func newTestHandlers(repo *fakeRepository) Handlers {
	return NewHandlers(issueopsdecisionapplication.NewService(
		repo,
		fixedClock{now: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)},
		identityPaths{},
	))
}

func sampleRequest() issueopsdecisioncontract.Request {
	return issueopsdecisioncontract.Request{
		Kind:  "architecture",
		Title: "delegate inbound mapping tests",
		Body:  "cover decision add paths through inbound handlers",
	}
}

func TestHandlersAddDelegatesToServiceAndAppliesDecision(t *testing.T) {
	repo := &fakeRepository{}
	handlers := newTestHandlers(repo)

	record, err := handlers.Add("/state", "io-7", sampleRequest())
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if repo.updateCalls != 1 || repo.stateRoot != "/state" || repo.id != "io-7" {
		t.Fatalf("repository received wrong call: calls=%d root=%q id=%q", repo.updateCalls, repo.stateRoot, repo.id)
	}
	if !record.OK || len(record.Decisions) != 1 {
		t.Fatalf("decision was not applied by service: %+v", record)
	}
}

func TestHandlersAddWithActorUsesSamePath(t *testing.T) {
	repo := &fakeRepository{}
	handlers := newTestHandlers(repo)

	actor := issueopsdecisioncontract.Actor{Host: "codex", SessionID: "s-1"}
	if _, err := handlers.AddWithActor("/state", "io-8", sampleRequest(), actor); err != nil {
		t.Fatalf("add with actor failed: %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repo.updateCalls)
	}
}

func TestHandlersAddRejectsInvalidRequestBeforeWrite(t *testing.T) {
	repo := &fakeRepository{}
	handlers := newTestHandlers(repo)

	if _, err := handlers.Add("/state", "io-9", issueopsdecisioncontract.Request{}); err == nil ||
		!strings.Contains(err.Error(), "invalid decision kind") {
		t.Fatalf("empty request must fail validation, got %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repository must not be written when validation fails, got %d calls", repo.updateCalls)
	}
}

func TestHandlersAddPropagatesRepositoryError(t *testing.T) {
	repo := &fakeRepository{err: context.DeadlineExceeded}
	handlers := newTestHandlers(repo)

	if _, err := handlers.Add("/state", "io-10", sampleRequest()); err == nil {
		t.Fatal("repository error must surface through handler")
	}
}

func TestNewHandlersWithoutServiceFailsClosed(t *testing.T) {
	handlers := NewHandlers(nil)
	if _, err := handlers.Add("/state", "io-11", sampleRequest()); err == nil ||
		!strings.Contains(err.Error(), "dependencies are required") {
		t.Fatalf("nil service must fail closed, got %v", err)
	}
}
