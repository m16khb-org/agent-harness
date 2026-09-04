package issueopspublication

import (
	"context"
	"fmt"

	application "issueops/internal/application/issueopspublication"
	contract "issueops/internal/contract/issueopspublication"
)

type VerifyCandidateFunc func(context.Context, contract.Intent, contract.Candidate) error
type VerifyLiveFunc func(context.Context, contract.Intent, string) error

type Verifier struct {
	candidate VerifyCandidateFunc
	live      VerifyLiveFunc
}

func NewVerifier(candidate VerifyCandidateFunc, live VerifyLiveFunc) *Verifier {
	return &Verifier{candidate: candidate, live: live}
}

func (v *Verifier) VerifyCandidate(ctx context.Context, intent contract.Intent, candidate contract.Candidate) error {
	if v == nil || v.candidate == nil {
		return fmt.Errorf("publication candidate verifier is required")
	}
	return v.candidate(ctx, intent.Clone(), candidate.Clone())
}

func (v *Verifier) VerifyLive(ctx context.Context, intent contract.Intent, url string) error {
	if v == nil || v.live == nil {
		return fmt.Errorf("publication live verifier is required")
	}
	return v.live(ctx, intent.Clone(), url)
}

var _ application.Verifier = (*Verifier)(nil)
