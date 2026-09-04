package guard

import guardcontract "issueops/internal/contract/guard"

// GuardBlockedError는 Error() 메서드를 가지므로 contract가 아니라 여기에 남는다.
// contract는 DTO만 소유하고 동작을 갖지 않는다.
type GuardBlockedError struct {
	Findings []guardcontract.GuardFinding
}

func (e GuardBlockedError) Error() string {
	if len(e.Findings) == 0 {
		return "guard check blocked"
	}
	return "guard check blocked: " + e.Findings[0].Rule
}

func IsGuardBlocked(err error) bool {
	_, ok := err.(GuardBlockedError)
	return ok
}
