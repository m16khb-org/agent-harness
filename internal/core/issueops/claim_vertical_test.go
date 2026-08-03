package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	"agent-harness/internal/adapter/outbound/sqlstore"
	leaseapp "agent-harness/internal/application/issueopslease"
	"agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

func claimViaVertical(stateRoot string, request ExecutionClaimRequest, _ ...func(issueops.IssueOpsRecord) error) (ExecutionResult, error) {
	return claimViaVerticalWithDeps(context.Background(), stateRoot, request, ExecutionClaimDependencies{})
}

func claimViaVerticalWithDeps(ctx context.Context, stateRoot string, request ExecutionClaimRequest, deps ExecutionClaimDependencies) (ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return ExecutionResult{ID: request.ID}, err
	}
	record, err := ReadIssueOps(stateRoot, request.ID)
	if err != nil {
		return ExecutionResult{ID: request.ID}, err
	}
	preflight := leaseoutbound.NewClaimContextPreflight(db, func(ctx context.Context, repo, issueURL string) (leaseoutbound.IssueSnapshot, error) {
		if deps.ReadIssue == nil {
			return leaseoutbound.IssueSnapshot{}, fmt.Errorf("remote issue snapshot reader is unavailable for the Orca claim")
		}
		snapshot, err := deps.ReadIssue(ctx, executionOwnerIssueProvider(record), port.ExecutionIssueSnapshotRequest{Repo: repo, URL: issueURL})
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		return leaseoutbound.IssueSnapshot{URL: snapshot.URL, Body: snapshot.Body}, nil
	})
	service := leaseapp.NewClaimService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, preflight)
	result, err := service.Claim(ctx, leaseapp.ClaimRequest{
		ID: request.ID, Generation: request.Generation, Actor: toVerticalActor(request.Actor), Ancestry: toVerticalAncestry(request.Actor),
		CWD: request.CWD, TokenFile: request.TokenFile, IssueBodySHA256: request.IssueBodySHA256, ContextPacketSHA256: request.ContextPacketSHA256,
	})
	if err != nil {
		return ExecutionResult{ID: request.ID}, publicVerticalClaimError(err, request.Generation)
	}
	var execution issueops.Execution
	data, err := json.Marshal(result.Execution)
	if err != nil {
		return ExecutionResult{ID: request.ID}, err
	}
	if err := json.Unmarshal(data, &execution); err != nil {
		return ExecutionResult{ID: request.ID}, err
	}
	return ExecutionResult{OK: result.OK, ID: result.ID, Execution: execution}, nil
}

func claimViaVerticalHandler(ctx context.Context, stateRoot string, request ExecutionClaimRequest, deps ExecutionClaimDependencies) (ExecutionResult, error) {
	return claimViaVerticalWithDeps(ctx, stateRoot, request, deps)
}

func toVerticalActor(actor issueops.NativeActor) leasedomain.Actor {
	result := leasedomain.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.Process = &leasedomain.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}
	}
	return result
}

func toVerticalAncestry(actor issueops.NativeActor) []leasedomain.ProcessReceipt {
	result := make([]leasedomain.ProcessReceipt, 0, len(actor.ProcessAncestry))
	for _, receipt := range actor.ProcessAncestry {
		result = append(result, leasedomain.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return result
}

func publicVerticalClaimError(err error, generation uint64) error {
	switch leasedomain.DenyCodeOf(err) {
	case leasedomain.DenyLeaseClaimable:
		return fmt.Errorf("lease is not claimable at generation %d", generation)
	case leasedomain.DenyCanonicalCWD:
		return fmt.Errorf("claim cwd must be the canonical worktree")
	case leasedomain.DenyClaimToken:
		return fmt.Errorf("claim token does not match the current generation")
	}
	var failure *leasecontract.Failure
	if errors.As(err, &failure) {
		return failure.Cause
	}
	return err
}
