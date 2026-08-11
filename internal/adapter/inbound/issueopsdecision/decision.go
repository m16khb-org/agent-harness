package issueopsdecision

import (
	"context"

	issueopsdecisionapplication "agent-harness/internal/application/issueopsdecision"
	issueopsdecisioncontract "agent-harness/internal/contract/issueopsdecision"
)

type Handlers struct {
	Add func(
		string,
		string,
		issueopsdecisioncontract.Request,
	) (issueopsdecisioncontract.Record, error)
	AddWithActor func(
		string,
		string,
		issueopsdecisioncontract.Request,
		issueopsdecisioncontract.Actor,
	) (issueopsdecisioncontract.Record, error)
}

func NewHandlers(service *issueopsdecisionapplication.Service) Handlers {
	return Handlers{
		Add: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
		) (issueopsdecisioncontract.Record, error) {
			return service.Add(context.Background(), stateRoot, id, request, nil)
		},
		AddWithActor: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
			actor issueopsdecisioncontract.Actor,
		) (issueopsdecisioncontract.Record, error) {
			return service.Add(context.Background(), stateRoot, id, request, &actor)
		},
	}
}
