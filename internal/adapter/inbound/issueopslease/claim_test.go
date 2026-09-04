package issueopslease

import (
	"errors"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

func TestClaimInboundContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "lease", err: leasedomain.Deny(leasedomain.DenyLeaseClaimable, errors.New("internal")), want: "lease is not claimable at generation 7: a released or superseded lease must be reseeded before any session can claim it; run `issueops execution replace --id io-demo --expected-generation 7 --preview` and follow the next_command it renders"},
		{name: "cwd", err: leasedomain.Deny(leasedomain.DenyCanonicalCWD, errors.New("internal")), want: "claim cwd must be the canonical worktree"},
		{name: "token", err: leasedomain.Deny(leasedomain.DenyClaimToken, errors.New("internal")), want: "claim token does not match the current generation"},
		{name: "persistence", err: leasecontract.Fail(leasecontract.FailurePersistence, errors.New("holder index is unavailable")), want: "holder index is unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicClaimError(tc.err, 7, "io-demo").Error(); got != tc.want {
				t.Fatalf("public error=%q want=%q", got, tc.want)
			}
		})
	}
}

// TestClaimClaimableDenyCarriesReseedRecovery는 정상 반납 뒤의 인계가 막히는
// 지점에 회복 경로가 붙어 있는지 본다.
//
// `execution release`는 lease를 `released`로 두는데, 그 상태는 claimable이
// 아니다. 다른 세션이 문서대로 `execution claim --claim-current-token`을
// 실행하면 `lease is not claimable at generation N`만 받고 멈춘다. 회복은
// `execution replace --preview` 다음 `--reseed --confirm`이며, 그 경로는
// `execution status`의 next_command에만 있었다. 실패한 claim 자체가 아무
// 안내도 주지 않으면, 문맥 없는 새 세션은 반납된 lease를 인수할 방법을
// 알아낼 수 없다.
func TestClaimClaimableDenyCarriesReseedRecovery(t *testing.T) {
	err := publicClaimError(leasedomain.Deny(leasedomain.DenyLeaseClaimable, errors.New("internal")), 3, "io-abc123")
	for _, want := range []string{
		"lease is not claimable at generation 3",
		"must be reseeded",
		"issueops execution replace --id io-abc123 --expected-generation 3 --preview",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("claim deny must mention %q; got %q", want, err.Error())
		}
	}
}
