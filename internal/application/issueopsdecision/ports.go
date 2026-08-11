package issueopsdecision

import (
	"context"
	"time"

	issueopsdecisioncontract "agent-harness/internal/contract/issueopsdecision"
)

type Repository interface {
	Update(
		context.Context,
		string,
		string,
		func(issueopsdecisioncontract.Record) (issueopsdecisioncontract.Record, error),
	) (issueopsdecisioncontract.Record, error)
}

type Clock interface {
	Now() time.Time
}

type PathMatcher interface {
	Same(string, string) bool
}
