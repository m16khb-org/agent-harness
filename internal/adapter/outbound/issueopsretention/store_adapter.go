package issueopsretention

import (
	"context"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
)

const artifactStageBucket = "artifact_stage_v1"

type Repository struct {
	Store issueopsrecord.Store
}

func (repository Repository) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	return repository.Store.ListIDs(ctx, stateRoot)
}

func (repository Repository) ReadUnchecked(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsretentioncontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}

func (repository Repository) Delete(ctx context.Context, stateRoot, id string) error {
	return repository.Store.Delete(ctx, stateRoot, id, artifactStageBucket)
}
