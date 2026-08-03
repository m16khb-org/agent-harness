package harnessapp

import (
	"context"

	leaseinbound "agent-harness/internal/adapter/inbound/issueopslease"
	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	"agent-harness/internal/adapter/outbound/sqlstore"
	leaseapp "agent-harness/internal/application/issueopslease"
	"agent-harness/internal/core/issueops"
)

func issueOpsReleaseHandler(ctx context.Context, stateRoot string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	service := leaseapp.NewReleaseService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, leaseoutbound.FilesystemPathMatcher{})
	return leaseinbound.NewReleaseHandler(service)(ctx, stateRoot, request)
}
