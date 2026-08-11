package harnessapp

import (
	issueopsstatusinbound "agent-harness/internal/adapter/inbound/issueopsstatus"
	issueopscore "agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsstatusoutbound "agent-harness/internal/adapter/outbound/issueopsstatus"
	issueopsstatusapplication "agent-harness/internal/application/issueopsstatus"
	issueopsstatuscontract "agent-harness/internal/contract/issueopsstatus"
	issueopsstatusdomain "agent-harness/internal/domain/issueopsstatus"
)

func issueOpsStatusHandler(
	observers ...issueopsrecord.Observer,
) func(
	string,
	string,
) (issueopsstatuscontract.Record, error) {
	service := issueopsstatusapplication.NewService(
		issueopsstatusoutbound.Repository{
			Store: issueOpsRecordStore("status", observers...),
		},
		issueopsstatusdomain.NewProjector(issueopscore.IssueOpsPhaseCompletion),
	)
	return issueopsstatusinbound.NewStatusHandler(service)
}
