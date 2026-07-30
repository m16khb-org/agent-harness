package issueopslease

import (
	"fmt"
	"time"
)

const (
	DenyLeaseClaimable DenyCode = "lease_claimable"
	DenyClaimToken     DenyCode = "claim_token"
)

type ClaimRequest struct {
	Generation        uint64
	Actor             Actor
	AuthorityVerified bool
	CanonicalCWD      bool
	TokenVerified     bool
}

type ClaimOutcome struct {
	Status     string
	Holder     *Actor
	ClaimedAt  string
	ReleasedAt string
}

func IsClaimRetry(lease Lease, generation uint64, actor Actor) bool {
	return lease.Status == "active" && lease.Generation == generation && sameActor(lease.Holder, &actor)
}

func ValidateClaim(lease Lease, request ClaimRequest) error {
	if lease.Status == "active" && lease.Generation == request.Generation && sameActor(lease.Holder, &request.Actor) {
		return nil
	}
	if !request.AuthorityVerified || lease.Status != "claimable" || lease.Generation != request.Generation {
		return Deny(DenyLeaseClaimable, fmt.Errorf("lease is not claimable at generation %d", request.Generation))
	}
	if !request.CanonicalCWD {
		return Deny(DenyCanonicalCWD, fmt.Errorf("claim cwd is not canonical"))
	}
	if !request.TokenVerified {
		return Deny(DenyClaimToken, fmt.Errorf("claim token does not match the current generation"))
	}
	return nil
}

func ApplyClaim(now time.Time, actor Actor) ClaimOutcome {
	holder := actor
	return ClaimOutcome{
		Status:    "active",
		Holder:    &holder,
		ClaimedAt: now.UTC().Format(time.RFC3339Nano),
	}
}
