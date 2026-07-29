package issueopslease

import (
	"context"
	"time"

	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/domain/issueopslease"
)

type Repository interface {
	Update(context.Context, string, RecordValidator, RecordTransition) (RepositoryResult, error)
}

type RecordValidator func(Record) error
type RecordTransition func(Record) (Record, error)

type Record struct {
	ID            string
	CanonicalRoot string
	Lease         leasecontract.Lease
}

// RepositoryResult은 같은 transaction에서 저장한 v1 execution projection이다.
// inbound adapter가 후속 status read로 다른 writer의 sidecar를 섞지 않게 한다.
type RepositoryResult struct {
	Record    Record
	Execution leasecontract.Execution
}

type Clock interface{ Now() time.Time }
type ProcessInspector func(context.Context, issueopslease.ProcessReceipt) (string, issueopslease.ProcessReceipt, error)
type CanonicalPathMatcher interface{ Matches(string, string) bool }
