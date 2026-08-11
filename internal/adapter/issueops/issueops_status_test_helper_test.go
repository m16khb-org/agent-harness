package issueops

import (
	"context"

	issueopsstatusoutbound "agent-harness/internal/adapter/outbound/issueopsstatus"
	issueopsstatusapplication "agent-harness/internal/application/issueopsstatus"
	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsstatusdomain "agent-harness/internal/domain/issueopsstatus"
)

func readIssueOpsStatusForTest(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
	service := issueopsstatusapplication.NewService(
		issueopsstatusoutbound.Repository{},
		issueopsstatusdomain.NewProjector(IssueOpsPhaseCompletion),
	)
	return service.Status(context.Background(), stateRoot, id)
}
