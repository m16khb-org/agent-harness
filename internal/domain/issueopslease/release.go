// Package issueopslease는 execution lease release의 순수 전이와 권한 판정을 소유한다.
package issueopslease

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type DenyCode string

const (
	DenyLeaseAuthority DenyCode = "lease_authority"
	DenyCanonicalCWD   DenyCode = "canonical_cwd"
)

type Denial struct {
	Code  DenyCode
	Cause error
}

func (e *Denial) Error() string             { return fmt.Sprintf("%s: %v", e.Code, e.Cause) }
func (e *Denial) Unwrap() error             { return e.Cause }
func Deny(code DenyCode, cause error) error { return &Denial{Code: code, Cause: cause} }
func DenyCodeOf(err error) DenyCode {
	var denial *Denial
	if errors.As(err, &denial) {
		return denial.Code
	}
	return ""
}

type Lease struct {
	Generation       uint64
	Status           string
	Holder           *Actor
	ClaimTokenSHA256 string
}
type Actor struct {
	Host      string
	SessionID string
	AgentID   string
	Process   *ProcessReceipt
}
type ProcessReceipt struct {
	PID        int
	StartedAt  string
	Executable string
}
type ReleaseRequest struct {
	Generation        uint64
	Actor             Actor
	AuthorityVerified bool
	CanonicalCWD      bool
}

func ValidateRelease(lease Lease, request ReleaseRequest) error {
	if !request.AuthorityVerified || lease.Status != "active" || lease.Generation != request.Generation || !sameActor(lease.Holder, &request.Actor) {
		return Deny(DenyLeaseAuthority, fmt.Errorf("requester is not the active generation holder"))
	}
	if !request.CanonicalCWD {
		return Deny(DenyCanonicalCWD, fmt.Errorf("release cwd is not canonical"))
	}
	return nil
}
func ApplyRelease(now time.Time) ReleaseOutcome {
	return ReleaseOutcome{Status: "released", ReleasedAt: now.UTC().Format(time.RFC3339Nano)}
}

type ReleaseOutcome struct {
	Status     string
	ReleasedAt string
}

func sameActor(left, right *Actor) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return strings.EqualFold(strings.TrimSpace(left.Host), strings.TrimSpace(right.Host)) && left.SessionID == strings.TrimSpace(right.SessionID) && left.AgentID == strings.TrimSpace(right.AgentID) && sameProcess(left.Process, right.Process)
}
func sameProcess(left, right *ProcessReceipt) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
