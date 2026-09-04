package issueopsstatus

import (
	"context"
	"fmt"

	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

type Service struct {
	repository Repository
	projector  Projector
}

func NewService(repository Repository, projector Projector) *Service {
	return &Service{repository: repository, projector: projector}
}

func (service *Service) Status(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsstatuscontract.Record, error) {
	if service == nil || service.repository == nil || service.projector == nil {
		return issueopsstatuscontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops status dependencies are required",
		)
	}
	record, err := service.repository.Read(ctx, stateRoot, id)
	if err != nil {
		return record, err
	}
	return service.projector.Project(record), nil
}
