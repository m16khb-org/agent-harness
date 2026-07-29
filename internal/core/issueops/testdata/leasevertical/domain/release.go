package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type DenyCode string

const (
	DenyLeaseAuthority    DenyCode = "lease_authority"
	DenyCanonicalCWD      DenyCode = "canonical_cwd"
	DenyMalformedSchema   DenyCode = "malformed_schema"
	DenyUnsupportedSchema DenyCode = "unsupported_schema"
	DenyPersistence       DenyCode = "persistence"
)

type Denial struct {
	Code  DenyCode
	Cause error
}

func (e *Denial) Error() string {
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *Denial) Unwrap() error {
	return e.Cause
}

func Deny(code DenyCode, cause error) error {
	return &Denial{Code: code, Cause: cause}
}

func DenyCodeOf(err error) DenyCode {
	var denial *Denial
	if errors.As(err, &denial) {
		return denial.Code
	}
	return ""
}

type Record struct {
	ID        string
	Execution Execution
}

type Execution struct {
	Mode      string
	Workspace Workspace
	Lease     Lease
}

type Workspace struct {
	SourceRoot     string
	Root           string
	Branch         string
	BaseHead       string
	ParentWorktree string
	Driver         string
	LinkedAt       string
}

type Lease struct {
	Generation        uint64
	Status            string
	Holder            *Actor
	ClaimTokenSHA256  string
	ClaimedAt         string
	ReleasedAt        string
	ReplacedAt        string
	ReplacementReason string
}

type Actor struct {
	Host      string
	SessionID string
	AgentID   string
	Process   *ProcessReceipt
	Ancestry  []ProcessReceipt
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

func Release(record Record, request ReleaseRequest, now time.Time) (Record, error) {
	if err := ValidateRelease(record, request); err != nil {
		return Record{}, err
	}
	return ApplyRelease(record, now), nil
}

func ValidateRelease(record Record, request ReleaseRequest) error {
	lease := record.Execution.Lease
	if !request.AuthorityVerified ||
		lease.Status != "active" || lease.Generation != request.Generation || !sameActor(lease.Holder, &request.Actor) {
		return Deny(DenyLeaseAuthority, fmt.Errorf("requester is not the active generation holder"))
	}
	if !request.CanonicalCWD {
		return Deny(DenyCanonicalCWD, fmt.Errorf("release cwd is not canonical"))
	}
	return nil
}

func ApplyRelease(record Record, now time.Time) Record {
	lease := record.Execution.Lease
	lease.Status = "released"
	lease.Holder = nil
	lease.ClaimTokenSHA256 = ""
	lease.ReleasedAt = now.UTC().Format(time.RFC3339Nano)
	record.Execution.Lease = lease
	return record
}

func sameActor(left, right *Actor) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return strings.EqualFold(strings.TrimSpace(left.Host), strings.TrimSpace(right.Host)) &&
		left.SessionID == strings.TrimSpace(right.SessionID) &&
		left.AgentID == strings.TrimSpace(right.AgentID) &&
		sameProcess(left.Process, right.Process)
}

func sameProcess(left, right *ProcessReceipt) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
