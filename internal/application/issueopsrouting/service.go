package issueopsrouting

import (
	"context"
	"fmt"

	issueopsroutingcontract "agent-harness/internal/contract/issueopsrouting"
	issueopsauthorizationdomain "agent-harness/internal/domain/issueopsauthorization"
	issueopsroutingdomain "agent-harness/internal/domain/issueopsrouting"
)

type Service struct {
	repository Repository
	clock      Clock
	paths      PathMatcher
}

func NewService(repository Repository, clock Clock, paths PathMatcher) *Service {
	return &Service{repository: repository, clock: clock, paths: paths}
}

func (service *Service) Record(
	ctx context.Context,
	stateRoot string,
	id string,
	phase string,
	skill string,
	actor issueopsroutingcontract.Actor,
) (issueopsroutingcontract.Record, error) {
	if service == nil || service.repository == nil || service.clock == nil || service.paths == nil {
		return issueopsroutingcontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops routing dependencies are required",
		)
	}
	entry, err := issueopsroutingdomain.NewEntry(phase, skill, service.clock.Now())
	if err != nil {
		return issueopsroutingcontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopsroutingcontract.Record,
	) (issueopsroutingcontract.Record, bool, error) {
		if err := issueopsauthorizationdomain.AuthorizeExecutionMutation(
			record,
			&actor,
			service.paths.Same,
		); err != nil {
			record.OK = false
			return record, false, err
		}
		return issueopsroutingdomain.Append(record, entry)
	})
}

func (service *Service) Score(
	ctx context.Context,
	stateRoot string,
	id string,
	expected []issueopsroutingcontract.Expected,
) (issueopsroutingcontract.Result, int, error) {
	if service == nil || service.repository == nil {
		return issueopsroutingcontract.Result{OK: false}, 0, fmt.Errorf(
			"issueops routing repository is required",
		)
	}
	record, err := service.repository.Read(ctx, stateRoot, id)
	if err != nil {
		return issueopsroutingcontract.Result{OK: false}, 0, err
	}
	return issueopsroutingdomain.ScoreRecord(record, expected), len(record.RoutingTrace), nil
}
