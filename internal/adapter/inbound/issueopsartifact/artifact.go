package issueopsartifact

import (
	"context"

	issueopsartifactapplication "agent-harness/internal/application/issueopsartifact"
	issueopsartifactcontract "agent-harness/internal/contract/issueopsartifact"
)

type Handlers struct {
	Stage   func(string, string, string, []byte) (issueopsartifactcontract.Record, error)
	Names   func(string, string) ([]string, error)
	Unstage func(string, string, string) (issueopsartifactcontract.Record, error)
}

func NewHandlers(service *issueopsartifactapplication.Service) Handlers {
	return Handlers{
		Stage: func(
			stateRoot string,
			id string,
			name string,
			content []byte,
		) (issueopsartifactcontract.Record, error) {
			return service.Stage(context.Background(), stateRoot, id, name, content)
		},
		Names: func(stateRoot, id string) ([]string, error) {
			return service.Names(context.Background(), stateRoot, id)
		},
		Unstage: func(
			stateRoot string,
			id string,
			name string,
		) (issueopsartifactcontract.Record, error) {
			return service.Unstage(context.Background(), stateRoot, id, name)
		},
	}
}
