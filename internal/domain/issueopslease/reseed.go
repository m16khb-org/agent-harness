package issueopslease

import (
	"fmt"
	"strings"
	"time"
)

const (
	DenyLeaseGeneration DenyCode = "lease_generation"
	DenyReseedLease     DenyCode = "reseed_lease"
)

type ReseedRequest struct {
	ExpectedGeneration uint64
	CanonicalCWD       bool
	Reason             string
}

type ReseedOutcome struct {
	Generation        uint64
	Status            string
	Holder            *Actor
	ClaimTokenSHA256  string
	ReplacedAt        string
	ReplacementReason string
}

func ValidateReseed(lease Lease, request ReseedRequest) error {
	if err := ValidateReseedGeneration(lease, request.ExpectedGeneration); err != nil {
		return err
	}
	if !request.CanonicalCWD {
		return Deny(DenyCanonicalCWD, fmt.Errorf("execution replace cwd must be source_root or the canonical worktree"))
	}
	if (lease.Status != "released" && lease.Status != "claimable") || lease.Holder != nil {
		return Deny(DenyReseedLease, fmt.Errorf("reseed requires a released or claimable lease"))
	}
	return nil
}

func ValidateReseedGeneration(lease Lease, expected uint64) error {
	if expected == 0 || lease.Generation != expected {
		return Deny(DenyLeaseGeneration, fmt.Errorf("stale lease generation: current=%d expected=%d", lease.Generation, expected))
	}
	return nil
}

func ApplyReseed(now time.Time, lease Lease, request ReseedRequest) ReseedOutcome {
	return ReseedOutcome{
		Generation:        lease.Generation + 1,
		Status:            "claimable",
		Holder:            nil,
		ClaimTokenSHA256:  "",
		ReplacedAt:        now.UTC().Format(time.RFC3339Nano),
		ReplacementReason: strings.TrimSpace(request.Reason),
	}
}
