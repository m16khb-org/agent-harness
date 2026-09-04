package issueopslease

import (
	"context"
	"testing"
	"time"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

func TestClaimServiceOrder(t *testing.T) {
	trace := []string{}
	actor := leasedomain.Actor{Host: "codex", SessionID: "claim-session", Process: &leasedomain.ProcessReceipt{PID: 9, StartedAt: "2026-07-30T00:00:00Z", Executable: "/codex"}}
	preflight := claimPreflightFunc(func(_ context.Context, request ClaimPreflightRequest) (RecordValidator, error) {
		trace = append(trace, "preflight")
		if request.ID != "io-claim-order" || request.Generation != 3 {
			t.Fatalf("preflight request=%#v", request)
		}
		return func(Record) error { return nil }, nil
	})
	repository := claimRepositoryFunc(func(_ context.Context, request ClaimRepositoryRequest) (RepositoryResult, error) {
		trace = append(trace, "repository")
		if request.Actor.Host != actor.Host || request.Actor.SessionID != actor.SessionID || request.Actor.Process == nil || *request.Actor.Process != *actor.Process || request.ValidateRecord == nil {
			t.Fatalf("repository request=%#v", request)
		}
		return RepositoryResult{Record: Record{ID: request.ID}, Execution: leasecontract.Execution{}}, nil
	})
	service := NewClaimService(repository, fixedClaimClock{at: time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)}, func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		trace = append(trace, "actor")
		return "live", receipt, nil
	}, preflight)
	if _, err := service.Claim(context.Background(), ClaimRequest{ID: "io-claim-order", Generation: 3, Actor: actor, Ancestry: []leasedomain.ProcessReceipt{*actor.Process}, CWD: "/canonical"}); err != nil {
		t.Fatalf("claim: %v", err)
	}
	want := []string{"preflight", "actor", "repository"}
	if len(trace) != len(want) {
		t.Fatalf("trace=%v want=%v", trace, want)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("trace=%v want=%v", trace, want)
		}
	}
}

type claimPreflightFunc func(context.Context, ClaimPreflightRequest) (RecordValidator, error)

func (f claimPreflightFunc) Preflight(ctx context.Context, request ClaimPreflightRequest) (RecordValidator, error) {
	return f(ctx, request)
}

type claimRepositoryFunc func(context.Context, ClaimRepositoryRequest) (RepositoryResult, error)

func (f claimRepositoryFunc) Claim(ctx context.Context, request ClaimRepositoryRequest) (RepositoryResult, error) {
	return f(ctx, request)
}

type fixedClaimClock struct{ at time.Time }

func (c fixedClaimClock) Now() time.Time { return c.at }
