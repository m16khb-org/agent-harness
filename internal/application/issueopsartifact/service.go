package issueopsartifact

import (
	"context"
	"fmt"
	"sort"

	issueopsartifactcontract "agent-harness/internal/contract/issueopsartifact"
	issueopsartifactdomain "agent-harness/internal/domain/issueopsartifact"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) Stage(
	ctx context.Context,
	stateRoot string,
	id string,
	name string,
	content []byte,
) (issueopsartifactcontract.Record, error) {
	if service == nil || service.repository == nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops artifact repository is required",
		)
	}
	name, err := issueopsartifactdomain.NormalizeName(name)
	if err != nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, err
	}
	if err := issueopsartifactdomain.ValidateContent(content); err != nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopsartifactcontract.Record,
		staged issueopsartifactcontract.Staged,
	) (issueopsartifactcontract.Staged, error) {
		if !issueopsartifactdomain.CanStage(record, name) {
			return staged, &issueopsartifactcontract.RecoveryError{ID: record.ID}
		}
		staged[name] = string(content)
		return staged, nil
	})
}

func (service *Service) Unstage(
	ctx context.Context,
	stateRoot string,
	id string,
	name string,
) (issueopsartifactcontract.Record, error) {
	if service == nil || service.repository == nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops artifact repository is required",
		)
	}
	name, err := issueopsartifactdomain.NormalizeName(name)
	if err != nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopsartifactcontract.Record,
		staged issueopsartifactcontract.Staged,
	) (issueopsartifactcontract.Staged, error) {
		if record.Execution != nil {
			return staged, fmt.Errorf(
				"staged artifacts are sealed after execution prepare and cannot be unstaged",
			)
		}
		delete(staged, name)
		return staged, nil
	})
}

func (service *Service) Names(
	ctx context.Context,
	stateRoot string,
	id string,
) ([]string, error) {
	if service == nil || service.repository == nil {
		return nil, fmt.Errorf("issueops artifact repository is required")
	}
	staged, err := service.repository.ReadStaged(ctx, stateRoot, id)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(staged))
	for name := range staged {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
