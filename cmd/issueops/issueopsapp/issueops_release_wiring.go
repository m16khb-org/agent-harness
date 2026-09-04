package issueopsapp

import (
	"context"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	"issueops/internal/adapter/issueops"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
)

func issueOpsReleaseHandler(ctx context.Context, stateRoot string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	service := leaseapp.NewReleaseService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, leaseoutbound.FilesystemPathMatcher{})
	return leaseinbound.NewReleaseHandler(service)(ctx, stateRoot, request)
}
