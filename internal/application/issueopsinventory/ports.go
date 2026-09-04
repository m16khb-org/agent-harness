package issueopsinventory

import (
	"context"
	"time"

	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

type Repository interface {
	Scan(
		context.Context,
		string,
	) ([]issueopsinventorycontract.Record, []issueopsinventorycontract.RecordDiagnostic, error)
}

type Clock interface {
	Now() time.Time
}

type PathNormalizer interface {
	Normalize(string) string
}
