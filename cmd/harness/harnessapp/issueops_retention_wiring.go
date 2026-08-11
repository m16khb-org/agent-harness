package harnessapp

import (
	"time"

	issueopsretentioninbound "agent-harness/internal/adapter/inbound/issueopsretention"
	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsretentionoutbound "agent-harness/internal/adapter/outbound/issueopsretention"
	issueopsretentionapplication "agent-harness/internal/application/issueopsretention"
	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
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
