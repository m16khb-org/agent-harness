package issueops

import (
	issueopsdecisioninbound "agent-harness/internal/adapter/inbound/issueopsdecision"
	issueopsauthorizationoutbound "agent-harness/internal/adapter/outbound/issueopsauthorization"
	issueopsdecisionoutbound "agent-harness/internal/adapter/outbound/issueopsdecision"
	issueopsdecisionapplication "agent-harness/internal/application/issueopsdecision"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func addIssueOpsDecisionForTest(
	stateRoot string,
	id string,
	request issueopscontract.IssueOpsDecisionRecordRequest,
) (issueopscontract.IssueOpsRecord, error) {
	service := issueopsdecisionapplication.NewService(
		issueopsdecisionoutbound.Repository{},
		issueopsdecisionoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	)
	return issueopsdecisioninbound.NewHandlers(service).Add(stateRoot, id, request)
}
