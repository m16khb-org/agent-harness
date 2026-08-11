package issueopsretention

import (
	"context"
	"time"

	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
)

type Repository interface {
	ListIDs(context.Context, string) ([]string, error)
	ReadUnchecked(context.Context, string, string) (issueopsretentioncontract.Record, error)
	Delete(context.Context, string, string) error
}

type Clock interface {
	Now() time.Time
}
