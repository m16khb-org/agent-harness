package issueopsdecision

import (
	"context"
	"fmt"

	issueopsdecisioncontract "issueops/internal/contract/issueopsdecision"
	issueopsauthorizationdomain "issueops/internal/domain/issueopsauthorization"
	issueopsdecisiondomain "issueops/internal/domain/issueopsdecision"
)

type Service struct {
	repository Repository
	clock      Clock
	paths      PathMatcher
}

func NewService(repository Repository, clock Clock, paths PathMatcher) *Service {
	return &Service{repository: repository, clock: clock, paths: paths}
}

func (service *Service) Add(
	ctx context.Context,
	stateRoot string,
	id string,
	request issueopsdecisioncontract.Request,
	actor *issueopsdecisioncontract.Actor,
) (issueopsdecisioncontract.Record, error) {
	if service == nil || service.repository == nil || service.clock == nil || service.paths == nil {
		return issueopsdecisioncontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops decision dependencies are required",
		)
	}
	decision, err := issueopsdecisiondomain.Build(request, service.clock.Now())
	if err != nil {
		return issueopsdecisioncontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopsdecisioncontract.Record,
	) (issueopsdecisioncontract.Record, error) {
		if err := issueopsauthorizationdomain.AuthorizeExecutionMutation(
			record,
			actor,
			service.paths.Same,
		); err != nil {
			record.OK = false
			return record, err
		}
		record.Decisions = append(record.Decisions, decision)
		record.UpdatedAt = decision.CreatedAt
		record.OK = true
		return record, nil
	})
}
