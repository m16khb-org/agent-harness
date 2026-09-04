package issueopsnext

import (
	"context"

	issueopsnextapplication "agent-harness/internal/application/issueopsnext"
	issueopsnextcontract "agent-harness/internal/contract/issueopsnext"
)

func NewNextHandler(service *issueopsnextapplication.Service) func(
	string,
	string,
	string,
) (issueopsnextcontract.Result, error) {
	return func(stateRoot, cwd, id string) (issueopsnextcontract.Result, error) {
		return service.Next(context.Background(), stateRoot, cwd, id)
	}
}
