package issueopslease

import (
	"strings"
	"testing"
	"time"
)

func TestClaimDomainContract(t *testing.T) {
	actor := Actor{Host: "codex", SessionID: "session", Process: &ProcessReceipt{PID: 1, StartedAt: "2026-07-30T00:00:00Z", Executable: "/codex"}}
	lease := Lease{Generation: 3, Status: "claimable"}
	request := ClaimRequest{Generation: 3, Actor: actor, AuthorityVerified: true, CanonicalCWD: true, TokenVerified: true}
	if err := ValidateClaim(lease, request); err != nil {
		t.Fatalf("validate claim: %v", err)
	}
	outcome := ApplyClaim(time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC), actor)
	if outcome.Status != "active" || outcome.Holder == nil || outcome.ClaimedAt == "" || outcome.ReleasedAt != "" {
		t.Fatalf("claim outcome=%#v", outcome)
	}
	for _, tc := range []struct {
		name    string
		request ClaimRequest
		code    DenyCode
	}{
		{name: "lease", request: ClaimRequest{Generation: 2, Actor: actor, AuthorityVerified: true, CanonicalCWD: true, TokenVerified: true}, code: DenyLeaseClaimable},
		{name: "cwd", request: ClaimRequest{Generation: 3, Actor: actor, AuthorityVerified: true, TokenVerified: true}, code: DenyCanonicalCWD},
		{name: "token", request: ClaimRequest{Generation: 3, Actor: actor, AuthorityVerified: true, CanonicalCWD: true}, code: DenyClaimToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClaim(lease, tc.request)
			if DenyCodeOf(err) != tc.code || !strings.Contains(err.Error(), string(tc.code)) {
				t.Fatalf("claim denial=%v code=%q", err, DenyCodeOf(err))
			}
		})
	}
}
