package issueopslease

import (
	"testing"
	"time"
)

func TestApplyReseedAdvancesHolderlessLease(t *testing.T) {
	holder := &Actor{Host: "codex", SessionID: "previous-holder"}
	outcome := ApplyReseed(time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC), Lease{Generation: 4, Status: "claimable", Holder: holder, ClaimTokenSHA256: "old-token-hash"}, ReseedRequest{
		ExpectedGeneration: 4,
		Reason:             "recover a holderless lease",
	})
	if outcome.Generation != 5 || outcome.Status != "claimable" || outcome.Holder != nil || outcome.ClaimTokenSHA256 != "" || outcome.ReplacedAt != "2026-07-30T08:00:00Z" || outcome.ReplacementReason != "recover a holderless lease" {
		t.Fatalf("outcome=%+v", outcome)
	}
}

func TestValidateReseedRejectsNonHolderlessOrStaleLease(t *testing.T) {
	tests := []struct {
		name     string
		lease    Lease
		expected uint64
		want     DenyCode
	}{
		{name: "active", lease: Lease{Generation: 4, Status: "active"}, expected: 4, want: DenyReseedLease},
		{name: "revoking", lease: Lease{Generation: 4, Status: "revoking"}, expected: 4, want: DenyReseedLease},
		{name: "released with holder", lease: Lease{Generation: 4, Status: "released", Holder: &Actor{Host: "codex", SessionID: "stale"}}, expected: 4, want: DenyReseedLease},
		{name: "claimable with holder", lease: Lease{Generation: 4, Status: "claimable", Holder: &Actor{Host: "codex", SessionID: "stale"}}, expected: 4, want: DenyReseedLease},
		{name: "stale", lease: Lease{Generation: 4, Status: "released"}, expected: 3, want: DenyLeaseGeneration},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateReseed(test.lease, ReseedRequest{ExpectedGeneration: test.expected, CanonicalCWD: true})
			if DenyCodeOf(err) != test.want {
				t.Fatalf("error=%v code=%q want=%q", err, DenyCodeOf(err), test.want)
			}
		})
	}
}

func TestValidateReseedAcceptsReleasedAndClaimable(t *testing.T) {
	for _, status := range []string{"released", "claimable"} {
		t.Run(status, func(t *testing.T) {
			err := ValidateReseed(Lease{Generation: 4, Status: status}, ReseedRequest{ExpectedGeneration: 4, CanonicalCWD: true})
			if err != nil {
				t.Fatalf("validate reseed: %v", err)
			}
		})
	}
}
