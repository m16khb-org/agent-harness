package issueopscompletion

import (
	"context"
	"time"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

type RecordTransition func(completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error)

type RepositoryResult struct {
	Record    completioncontract.RecordSnapshot
	Execution completioncontract.Execution
}

type Repository interface {
	Update(context.Context, string, RecordTransition) (RepositoryResult, error)
}

type Environment interface {
	VerifyArtifact(completioncontract.RecordSnapshot, string) error
	PathsMatch(string, string) bool
	CurrentHead(context.Context, string) (string, error)
	VerifyReport(string, string) (string, error)
}

type Clock interface{ Now() time.Time }

type ProcessInspector func(context.Context, completioncontract.ProcessReceipt) (string, completioncontract.ProcessReceipt, error)
