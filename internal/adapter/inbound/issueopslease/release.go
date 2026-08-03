// Package issueopslease maps execution requests to the release use case.
package issueopslease

import (
	"context"
	"errors"
	"fmt"

	leaseapp "agent-harness/internal/application/issueopslease"
	issueopscontract "agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/issueops"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

type ReleaseHandler struct{ service *leaseapp.ReleaseService }

func NewReleaseHandler(service *leaseapp.ReleaseService) issueops.ExecutionReleaseHandler {
	handler := ReleaseHandler{service: service}
	return handler.Handle
}

func (h ReleaseHandler) Handle(ctx context.Context, _ string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
	if h.service == nil {
		return issueops.ExecutionResult{ID: request.ID}, issueops.ErrReleaseHandlerUnavailable
	}
	result, err := h.service.Release(ctx, leaseapp.ReleaseRequest{ID: request.ID, Generation: request.Generation, Actor: toDomainActor(request.Actor), Ancestry: toProcessAncestry(request.Actor), CWD: request.CWD})
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, publicReleaseError(err, request.Generation)
	}
	return issueops.ExecutionResult{OK: result.OK, ID: result.ID, Execution: toCoreExecution(result.Execution)}, nil
}

func toCoreExecution(execution leasecontract.Execution) issueopscontract.Execution {
	result := issueopscontract.Execution{
		Mode: issueopscontract.ExecutionMode(execution.Mode),
		Workspace: issueopscontract.Workspace{
			SourceRoot: execution.Workspace.SourceRoot, Root: execution.Workspace.Root, Branch: execution.Workspace.Branch,
			BaseHead: execution.Workspace.BaseHead, ParentWorktree: execution.Workspace.ParentWorktree, Driver: execution.Workspace.Driver, LinkedAt: execution.Workspace.LinkedAt,
		},
		Lease: toCoreLease(execution.Lease),
	}
	if execution.Orca != nil {
		result.Orca = &issueopscontract.OrcaBinding{RuntimeID: execution.Orca.RuntimeID, RepoID: execution.Orca.RepoID, WorktreeID: execution.Orca.WorktreeID, RunID: execution.Orca.RunID, WorktreeInstanceID: execution.Orca.WorktreeInstanceID, LeaseGeneration: execution.Orca.LeaseGeneration, ArtifactIdentityVersion: execution.Orca.ArtifactIdentityVersion, IssueBodySHA256: execution.Orca.IssueBodySHA256, ContextPacketSHA256: execution.Orca.ContextPacketSHA256, OwnerPromptSHA256: execution.Orca.OwnerPromptSHA256, OwnerHost: execution.Orca.OwnerHost, OwnerModel: execution.Orca.OwnerModel, OwnerEffort: execution.Orca.OwnerEffort, TaskID: execution.Orca.TaskID, DispatchID: execution.Orca.DispatchID, TerminalPTYID: execution.Orca.TerminalPTYID}
	}
	if execution.Pending != nil {
		result.Pending = &issueopscontract.ExternalIntent{OperationID: execution.Pending.OperationID, Kind: execution.Pending.Kind, Marker: execution.Pending.Marker, StartedAt: execution.Pending.StartedAt}
	}
	if execution.Completion != nil {
		result.Completion = &issueopscontract.ExecutionCompletion{FinalHead: execution.Completion.FinalHead, TuringReportPath: execution.Completion.TuringReportPath, Verification: append([]string(nil), execution.Completion.Verification...), RemoteArtifactURL: execution.Completion.RemoteArtifactURL, CompletedAt: execution.Completion.CompletedAt}
	}
	if execution.Failure != nil {
		result.Failure = &issueopscontract.ExecutionFailure{OperationID: execution.Failure.OperationID, Code: execution.Failure.Code, Message: execution.Failure.Message, At: execution.Failure.At}
	}
	for _, event := range execution.SyncBaseEvents {
		result.SyncBaseEvents = append(result.SyncBaseEvents, issueopscontract.ExecutionSyncBaseEvent{Mode: event.Mode, BaseBranch: event.BaseBranch, BaseOID: event.BaseOID, MergeCommit: event.MergeCommit, ConflictFiles: event.ConflictFiles, Actor: event.Actor, At: event.At})
	}
	return result
}

func toCoreLease(lease leasecontract.Lease) issueopscontract.WriteLease {
	result := issueopscontract.WriteLease{Generation: lease.Generation, Status: issueopscontract.LeaseStatus(lease.Status), ClaimTokenSHA256: lease.ClaimTokenSHA256, ClaimedAt: lease.ClaimedAt, ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt, ReplacementReason: lease.ReplacementReason}
	if lease.Holder != nil {
		result.Holder = &issueopscontract.NativeActor{Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID}
		if lease.Holder.SessionProcess != nil {
			result.Holder.SessionProcess = &issueopscontract.NativeProcessReceipt{PID: lease.Holder.SessionProcess.PID, StartedAt: lease.Holder.SessionProcess.StartedAt, Executable: lease.Holder.SessionProcess.Executable}
		}
	}
	return result
}

func publicReleaseError(err error, generation uint64) error {
	if code := leasedomain.DenyCodeOf(err); code == leasedomain.DenyLeaseAuthority {
		return fmt.Errorf("only the current holder may release generation %d", generation)
	} else if code == leasedomain.DenyCanonicalCWD {
		return fmt.Errorf("release cwd must be the canonical worktree")
	}
	var failure *leasecontract.Failure
	if !errors.As(err, &failure) {
		return err
	}
	return failure.Cause
}

func toDomainActor(actor issueopscontract.NativeActor) leasedomain.Actor {
	result := leasedomain.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.Process = &leasedomain.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}
	}
	return result
}

func toProcessAncestry(actor issueopscontract.NativeActor) []leasedomain.ProcessReceipt {
	ancestry := make([]leasedomain.ProcessReceipt, 0, len(actor.ProcessAncestry))
	for _, receipt := range actor.ProcessAncestry {
		ancestry = append(ancestry, leasedomain.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return ancestry
}
