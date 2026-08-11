package harnessapp

import (
	issueopsdecisioninbound "agent-harness/internal/adapter/inbound/issueopsdecision"
	issueopsauthorizationoutbound "agent-harness/internal/adapter/outbound/issueopsauthorization"
	issueopsdecisionoutbound "agent-harness/internal/adapter/outbound/issueopsdecision"
	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsdecisionapplication "agent-harness/internal/application/issueopsdecision"
)

func issueOpsDecisionHandlers(
	observers ...issueopsrecord.Observer,
) issueopsdecisioninbound.Handlers {
	service := issueopsdecisionapplication.NewService(
		issueopsdecisionoutbound.Repository{
			Store: issueOpsRecordStore("decision", observers...),
		},
		issueopsdecisionoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	)
	return issueopsdecisioninbound.NewHandlers(service)
}
