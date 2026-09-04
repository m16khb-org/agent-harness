package issueopsapp

import (
	"context"
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	"issueops/internal/adapter/issueops"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	"issueops/internal/port"
)

func issueOpsClaimHandler(ctx context.Context, stateRoot string, request issueops.ExecutionClaimRequest, deps issueops.ExecutionClaimDependencies) (issueops.ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	preflight := leaseoutbound.NewClaimContextPreflight(db, func(ctx context.Context, repo, issueURL string) (leaseoutbound.IssueSnapshot, error) {
		record, err := issueops.ReadIssueOps(stateRoot, request.ID)
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		providerName, err := issueOpsClaimProviderName(record)
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		if deps.ReadIssue == nil {
			return leaseoutbound.IssueSnapshot{}, fmt.Errorf("remote issue snapshot reader is unavailable for the Orca claim")
		}
		snapshot, err := deps.ReadIssue(ctx, providerName, port.ExecutionIssueSnapshotRequest{Repo: repo, URL: issueURL})
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		return leaseoutbound.IssueSnapshot{URL: snapshot.URL, Body: snapshot.Body}, nil
	})
	service := leaseapp.NewClaimService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, preflight)
	return leaseinbound.NewClaimHandler(service)(ctx, stateRoot, request, deps)
}

func issueOpsClaimProviderName(record issueopscontract.IssueOpsRecord) (string, error) {
	if record.BranchPrepare != nil {
		switch providerName := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider)); providerName {
		case "github", "gitlab":
			return providerName, nil
		}
	}
	return "", fmt.Errorf("linked issue provider is unavailable")
}
