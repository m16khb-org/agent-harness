package issueopspublication

import (
	"context"
	"errors"
	"testing"

	contract "issueops/internal/contract/issueopspublication"
)

func TestVerifierDelegatesExactCandidateAndLiveChecks(t *testing.T) {
	events := []string{}
	verifier := NewVerifier(
		func(_ context.Context, intent contract.Intent, candidate contract.Candidate) error {
			events = append(events, "candidate")
			if intent.OperationID != "op-1" || candidate.URL != "candidate" {
				t.Fatalf("intent=%#v candidate=%#v", intent, candidate)
			}
			return nil
		},
		func(_ context.Context, intent contract.Intent, url string) error {
			events = append(events, "live")
			if intent.OperationID != "op-1" || url != "candidate" {
				t.Fatalf("intent=%#v url=%q", intent, url)
			}
			return nil
		},
	)
	intent := contract.Intent{OperationID: "op-1"}
	if err := verifier.VerifyCandidate(context.Background(), intent, contract.Candidate{URL: "candidate"}); err != nil {
		t.Fatal(err)
	}
	if err := verifier.VerifyLive(context.Background(), intent, "candidate"); err != nil {
		t.Fatal(err)
	}
	if got := stringsJoin(events); got != "candidate,live" {
		t.Fatalf("events=%q", got)
	}
}

func TestVerifierRejectsMissingChecks(t *testing.T) {
	verifier := NewVerifier(nil, nil)
	if err := verifier.VerifyCandidate(context.Background(), contract.Intent{}, contract.Candidate{}); err == nil || err.Error() != "publication candidate verifier is required" {
		t.Fatalf("candidate err=%v", err)
	}
	if err := verifier.VerifyLive(context.Background(), contract.Intent{}, ""); err == nil || err.Error() != "publication live verifier is required" {
		t.Fatalf("live err=%v", err)
	}
}

func TestVerifierPreservesPrimaryError(t *testing.T) {
	cause := errors.New("mismatch")
	verifier := NewVerifier(func(context.Context, contract.Intent, contract.Candidate) error { return cause }, func(context.Context, contract.Intent, string) error { return cause })
	if err := verifier.VerifyCandidate(context.Background(), contract.Intent{}, contract.Candidate{}); err != cause {
		t.Fatalf("candidate err=%v", err)
	}
	if err := verifier.VerifyLive(context.Background(), contract.Intent{}, "url"); err != cause {
		t.Fatalf("live err=%v", err)
	}
}
