package issueopsapp

import (
	"time"

	issueopsretentioninbound "issueops/internal/adapter/inbound/issueopsretention"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsretentionoutbound "issueops/internal/adapter/outbound/issueopsretention"
	issueopsretentionapplication "issueops/internal/application/issueopsretention"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
)

func issueOpsRetentionPruneHandler(
	observers ...issueopsrecord.Observer,
) func(
	string,
	time.Duration,
	bool,
) (issueopsretentioncontract.Result, error) {
	service := issueopsretentionapplication.NewService(
		issueopsretentionoutbound.Repository{
			Store: issueOpsRecordStore("retention", observers...),
		},
		issueopsretentionoutbound.SystemClock{},
	)
	return issueopsretentioninbound.NewPruneHandler(service)
}
