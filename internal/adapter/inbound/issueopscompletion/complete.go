// Package issueopscompletion maps execution requests to the completion
// application service.
package issueopscompletion

import (
	"context"
	"errors"

	completionapp "agent-harness/internal/application/issueopscompletion"
	issueopscontract "agent-harness/internal/contract/issueops"
	completioncontract "agent-harness/internal/contract/issueopscompletion"
	leasecontract "agent-harness/internal/contract/issueopslease"
	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/core/issueops"
)

type service interface {
	Complete(context.Context, completionapp.Request) (completionapp.Result, error)
}

type Handler struct{ service service }

func NewHandler(service service) issueops.ExecutionCompleteHandler {
	return Handler{service: service}.Handle
}

func (h Handler) Handle(ctx context.Context, _ string, request issueops.ExecutionCompleteRequest) (issueops.ExecutionResult, error) {
	if h.service == nil {
		return issueops.ExecutionResult{ID: request.ID}, issueops.ErrCompleteHandlerUnavailable
	}
	result, err := h.service.Complete(ctx, completionapp.Request{
		ID: request.ID, Generation: request.Generation, Actor: completionActor(request.Actor), Ancestry: completionAncestry(request.Actor),
		CWD: request.CWD, FinalHead: request.FinalHead, TuringReportPath: request.TuringReportPath,
		Verification: append([]string(nil), request.Verification...), RemoteArtifactURL: request.RemoteArtifactURL, Confirm: request.Confirm,
	})
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, publicError(err)
	}
	return issueops.ExecutionResult{OK: result.OK, ID: result.ID, Execution: coreExecution(result.Execution), OrcaTaskSettled: result.OrcaTaskSettled, OrcaTaskError: result.OrcaTaskError}, nil
}

func completionActor(actor issueopscontract.NativeActor) completioncontract.Actor {
	result := completioncontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.Process = &completioncontract.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}
	}
	return result
}

func completionAncestry(actor issueopscontract.NativeActor) []completioncontract.ProcessReceipt {
	result := make([]completioncontract.ProcessReceipt, 0, len(actor.ProcessAncestry))
	for _, receipt := range actor.ProcessAncestry {
		result = append(result, completioncontract.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return result
}

func coreExecution(execution leasecontract.Execution) issueopscontract.Execution {
	result := issueopscontract.Execution{
		Mode: issueopscontract.ExecutionMode(execution.Mode),
		Workspace: issueopscontract.Workspace{
			SourceRoot: execution.Workspace.SourceRoot, Root: execution.Workspace.Root, Branch: execution.Workspace.Branch,
			BaseHead: execution.Workspace.BaseHead, ParentWorktree: execution.Workspace.ParentWorktree, Driver: execution.Workspace.Driver, LinkedAt: execution.Workspace.LinkedAt,
		},
		Lease: coreLease(execution.Lease),
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

func coreLease(lease leasecontract.Lease) issueopscontract.WriteLease {
	result := issueopscontract.WriteLease{Generation: lease.Generation, Status: issueopscontract.LeaseStatus(lease.Status), ClaimTokenSHA256: lease.ClaimTokenSHA256, ClaimedAt: lease.ClaimedAt, ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt, ReplacementReason: lease.ReplacementReason}
	if lease.Holder != nil {
		result.Holder = &issueopscontract.NativeActor{Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID}
		if lease.Holder.SessionProcess != nil {
			result.Holder.SessionProcess = &issueopscontract.NativeProcessReceipt{PID: lease.Holder.SessionProcess.PID, StartedAt: lease.Holder.SessionProcess.StartedAt, Executable: lease.Holder.SessionProcess.Executable}
		}
	}
	return result
}

func publicError(err error) error {
	if errors.Is(err, statecontract.ErrInvalidState) {
		return statecontract.ErrInvalidState
	}
	return err
}
