package issueopslease

import (
	"errors"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

func TestClaimInboundContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "lease", err: leasedomain.Deny(leasedomain.DenyLeaseClaimable, errors.New("internal")), want: "lease is not claimable at generation 7"},
		{name: "cwd", err: leasedomain.Deny(leasedomain.DenyCanonicalCWD, errors.New("internal")), want: "claim cwd must be the canonical worktree"},
		{name: "token", err: leasedomain.Deny(leasedomain.DenyClaimToken, errors.New("internal")), want: "claim token does not match the current generation"},
		{name: "persistence", err: leasecontract.Fail(leasecontract.FailurePersistence, errors.New("holder index is unavailable")), want: "holder index is unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicClaimError(tc.err, 7).Error(); got != tc.want {
				t.Fatalf("public error=%q want=%q", got, tc.want)
			}
		})
	}
}
