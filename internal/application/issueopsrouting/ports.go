package issueopsrouting

import (
	"context"
	"time"

	issueopsroutingcontract "agent-harness/internal/contract/issueopsrouting"
)

type Repository interface {
	Read(context.Context, string, string) (issueopsroutingcontract.Record, error)
	Update(
		context.Context,
		string,
		string,
		func(issueopsroutingcontract.Record) (issueopsroutingcontract.Record, bool, error),
	) (issueopsroutingcontract.Record, error)
}

type Clock interface {
	Now() time.Time
}

type PathMatcher interface {
	Same(string, string) bool
}
