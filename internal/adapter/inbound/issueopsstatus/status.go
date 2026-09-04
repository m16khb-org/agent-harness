package issueopsstatus

import (
	"context"

	issueopsstatusapplication "issueops/internal/application/issueopsstatus"
	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

func NewStatusHandler(service *issueopsstatusapplication.Service) func(
	string,
	string,
) (issueopsstatuscontract.Record, error) {
	return func(stateRoot, id string) (issueopsstatuscontract.Record, error) {
		return service.Status(context.Background(), stateRoot, id)
	}
}
