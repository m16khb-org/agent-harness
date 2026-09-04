package issueopsretention

import (
	"context"
	"time"

	issueopsretentionapplication "issueops/internal/application/issueopsretention"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
)

func NewPruneHandler(service *issueopsretentionapplication.Service) func(
	string,
	time.Duration,
	bool,
) (issueopsretentioncontract.Result, error) {
	return func(stateRoot string, maxAge time.Duration, confirm bool) (issueopsretentioncontract.Result, error) {
		return service.Prune(context.Background(), stateRoot, maxAge, confirm)
	}
}
