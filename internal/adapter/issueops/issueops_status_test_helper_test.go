package issueops

import (
	"context"

	issueopsstatusoutbound "issueops/internal/adapter/outbound/issueopsstatus"
	issueopsstatusapplication "issueops/internal/application/issueopsstatus"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsstatusdomain "issueops/internal/domain/issueopsstatus"
)

func readIssueOpsStatusForTest(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
	service := issueopsstatusapplication.NewService(
		issueopsstatusoutbound.Repository{},
		issueopsstatusdomain.NewProjector(IssueOpsPhaseCompletion),
	)
	return service.Status(context.Background(), stateRoot, id)
}
