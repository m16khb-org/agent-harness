package issueopsrouting

import (
	"context"

	issueopsroutingapplication "issueops/internal/application/issueopsrouting"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
)

type Handlers struct {
	Record func(
		string,
		string,
		string,
		string,
		issueopsroutingcontract.Actor,
	) (issueopsroutingcontract.Record, error)
	Score func(
		string,
		string,
		[]issueopsroutingcontract.Expected,
	) (issueopsroutingcontract.Result, int, error)
}

func NewHandlers(service *issueopsroutingapplication.Service) Handlers {
	return Handlers{
		Record: func(
			stateRoot string,
			id string,
			phase string,
			skill string,
			actor issueopsroutingcontract.Actor,
		) (issueopsroutingcontract.Record, error) {
			return service.Record(context.Background(), stateRoot, id, phase, skill, actor)
		},
		Score: func(
			stateRoot string,
			id string,
			expected []issueopsroutingcontract.Expected,
		) (issueopsroutingcontract.Result, int, error) {
			return service.Score(context.Background(), stateRoot, id, expected)
		},
	}
}
