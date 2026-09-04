package issueopsinventory

import (
	"context"

	issueopsapplication "issueops/internal/application/issueopsinventory"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func NewListHandler(service *issueopsapplication.Service) func(
	string,
	string,
) (issueopsinventorycontract.ListResult, error) {
	return func(stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
		return service.ListCycles(context.Background(), stateRoot, repo)
	}
}
