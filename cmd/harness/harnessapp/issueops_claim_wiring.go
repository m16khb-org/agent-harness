package harnessapp

import (
	"context"
	"fmt"
	"strings"

	leaseinbound "agent-harness/internal/adapter/inbound/issueopslease"
	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	"agent-harness/internal/adapter/provider"
	leaseapp "agent-harness/internal/application/issueopslease"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func issueOpsClaimHandler(ctx context.Context, stateRoot string, request issueops.ExecutionClaimRequest) (issueops.ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	preflight := leaseoutbound.NewClaimContextPreflight(db, func(ctx context.Context, repo, issueURL string) (leaseoutbound.IssueSnapshot, error) {
		record, err := issueops.ReadIssueOps(stateRoot, request.ID)
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		providerName, err := issueOpsClaimProviderName(record, issueURL)
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		snapshot, err := provider.ReadExecutionIssueSnapshot(ctx, providerName, port.ExecutionIssueSnapshotRequest{Repo: repo, URL: issueURL})
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		return leaseoutbound.IssueSnapshot{URL: snapshot.URL, Body: snapshot.Body}, nil
	})
	service := leaseapp.NewClaimService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, preflight)
	return leaseinbound.NewClaimHandler(service)(ctx, stateRoot, request)
}

func issueOpsClaimProviderName(record issueops.IssueOpsRecord, issueURL string) (string, error) {
	if record.BranchPrepare != nil {
		switch providerName := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider)); providerName {
		case "github", "gitlab":
			return providerName, nil
		}
	}
	url := strings.ToLower(strings.TrimSpace(issueURL))
	switch {
	case strings.Contains(url, "github"):
		return "github", nil
	case strings.Contains(url, "gitlab"):
		return "gitlab", nil
	default:
		return "", fmt.Errorf("linked issue provider is unavailable")
	}
}
