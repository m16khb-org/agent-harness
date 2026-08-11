package issueopsinventory

import (
	"context"

	issueopsapplication "agent-harness/internal/application/issueopsinventory"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

func NewListHandler(service *issueopsapplication.Service) func(
	string,
	string,
) (issueopsinventorycontract.ListResult, error) {
	return func(stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
		return service.ListCycles(context.Background(), stateRoot, repo)
	}
}
