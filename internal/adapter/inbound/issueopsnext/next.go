package issueopsnext

import (
	"context"

	issueopsnextapplication "issueops/internal/application/issueopsnext"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
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
