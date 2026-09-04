package issueops

import (
	issueopsdecisioninbound "issueops/internal/adapter/inbound/issueopsdecision"
	issueopsauthorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	issueopsdecisionoutbound "issueops/internal/adapter/outbound/issueopsdecision"
	issueopsdecisionapplication "issueops/internal/application/issueopsdecision"
	issueopscontract "issueops/internal/contract/issueops"
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
