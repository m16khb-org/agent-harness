package issueopslease

import (
	"errors"
	"fmt"
)

var ErrExecutionNotPrepared = errors.New("IssueOps execution v1 is not prepared")

// FailureCode는 persisted JSON과 저장소 경계에서만 생기는 실패를 구분한다.
// lease 권한과 canonical CWD는 순수 domain 전이가 소유한다.
type FailureCode string

const (
	FailureInvalidState FailureCode = "invalid_state"
	FailurePersistence  FailureCode = "persistence"
)

type Failure struct {
	Code  FailureCode
	Cause error
}

func (e *Failure) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Cause) }
func (e *Failure) Unwrap() error { return e.Cause }

func Fail(code FailureCode, cause error) error {
	return &Failure{Code: code, Cause: cause}
}

func FailureCodeOf(err error) FailureCode {
	if failure, ok := errors.AsType[*Failure](err); ok {
		return failure.Code
	}
	return ""
}
