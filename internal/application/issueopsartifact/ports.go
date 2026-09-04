package issueopsartifact

import (
	"context"

	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
)

type Repository interface {
	Update(
		context.Context,
		string,
		string,
		func(
			issueopsartifactcontract.Record,
			issueopsartifactcontract.Staged,
		) (issueopsartifactcontract.Staged, error),
	) (issueopsartifactcontract.Record, error)
	ReadStaged(context.Context, string, string) (issueopsartifactcontract.Staged, error)
}
