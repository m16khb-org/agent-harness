package issueopsinventory

import (
	"context"
	"time"

	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

type Repository interface {
	ListIDs(context.Context, string) ([]string, error)
	ReadUnchecked(context.Context, string, string) (issueopsinventorycontract.Record, error)
}

type Clock interface {
	Now() time.Time
}

type PathNormalizer interface {
	Normalize(string) string
}
