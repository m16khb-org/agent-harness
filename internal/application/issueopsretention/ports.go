package issueopsretention

import (
	"context"
	"time"

	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
)

type Repository interface {
	ListIDs(context.Context, string) ([]string, error)
	ReadUnchecked(context.Context, string, string) (issueopsretentioncontract.Record, error)
	DeleteIfUnchanged(context.Context, string, string, issueopsretentioncontract.Record) error
}

type Clock interface {
	Now() time.Time
}
