// Package issueopspreparation maps the compatibility execution facade to the
// preparation application service.
package issueopspreparation

import (
	"context"
	"encoding/json"
	"errors"

	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
)

type service interface {
	Prepare(context.Context, preparationcontract.Command) (preparationcontract.Result, error)
}

type Handler struct{ service service }

func NewHandler(service service) issueops.ExecutionPrepareHandler {
	return Handler{service: service}.Handle
}

func (handler Handler) Handle(ctx context.Context, _ string, request issueops.ExecutionPrepareRequest) (issueops.ExecutionPrepareResult, error) {
	if handler.service == nil {
		return issueops.ExecutionPrepareResult{ID: request.ID}, issueops.ErrPrepareHandlerUnavailable
	}
	result, err := handler.service.Prepare(ctx, preparationcontract.Command{
		ID: request.ID, Mode: request.Mode, Actor: preparationActor(request.Actor), CWD: request.CWD,
		OwnerHost: request.OwnerHost, OwnerModel: request.OwnerModel, OwnerEffort: request.OwnerEffort, Confirm: request.Confirm,
	})
	return coreResult(result), publicError(err)
}

func preparationActor(actor issueops.NativeActor) preparationcontract.Actor {
	result := preparationcontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	return result
}

func coreResult(result preparationcontract.Result) issueops.ExecutionPrepareResult {
	return issueops.ExecutionPrepareResult{
		OK: result.OK, ID: result.ID, Preview: result.Preview,
		RequestedMode: result.RequestedMode, ResolvedMode: result.ResolvedMode, FallbackCode: result.FallbackCode,
		Workspace: coreWorkspace(result.Workspace), Execution: coreExecution(result.Execution),
		ClaimTokenPath: result.ClaimTokenPath, IssueBodySHA256: result.IssueBodySHA256,
		ContextPacketPath: result.ContextPacketPath, ContextPacketSHA256: result.ContextPacketSHA256,
		OwnerPromptPath: result.OwnerPromptPath, OwnerPromptSHA256: result.OwnerPromptSHA256,
		IssueSnapshotSource: result.IssueSnapshotSource, NextCommand: result.NextCommand,
	}
}

func coreWorkspace(workspace preparationcontract.Workspace) model.Workspace {
	return model.Workspace{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
		BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree, Driver: workspace.Driver, LinkedAt: workspace.LinkedAt,
	}
}

func coreExecution(execution *leasecontract.Execution) *model.Execution {
	if execution == nil {
		return nil
	}
	result := &model.Execution{
		Mode: model.ExecutionMode(execution.Mode), Workspace: coreWorkspace(execution.Workspace),
		Lease: model.WriteLease{
			Generation: execution.Lease.Generation, Status: model.LeaseStatus(execution.Lease.Status),
			ClaimTokenSHA256: execution.Lease.ClaimTokenSHA256, ClaimedAt: execution.Lease.ClaimedAt,
			ReleasedAt: execution.Lease.ReleasedAt, ReplacedAt: execution.Lease.ReplacedAt, ReplacementReason: execution.Lease.ReplacementReason,
		},
	}
	if execution.Lease.Holder != nil {
		holder := execution.Lease.Holder
		result.Lease.Holder = &model.NativeActor{Host: holder.Host, SessionID: holder.SessionID, AgentID: holder.AgentID}
		if holder.SessionProcess != nil {
			result.Lease.Holder.SessionProcess = &model.NativeProcessReceipt{
				PID: holder.SessionProcess.PID, StartedAt: holder.SessionProcess.StartedAt, Executable: holder.SessionProcess.Executable,
			}
		}
	}
	if execution.Orca != nil {
		binding := execution.Orca
		result.Orca = &model.OrcaBinding{
			RuntimeID: binding.RuntimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
			RunID: binding.RunID, WorktreeInstanceID: binding.WorktreeInstanceID, LeaseGeneration: binding.LeaseGeneration,
			OwnerHost: binding.OwnerHost, OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
			TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		}
	}
	if execution.Pending != nil {
		pending := execution.Pending
		result.Pending = &model.ExternalIntent{OperationID: pending.OperationID, Kind: pending.Kind, Marker: pending.Marker, StartedAt: pending.StartedAt}
	}
	if execution.Completion != nil {
		completion := execution.Completion
		result.Completion = &model.ExecutionCompletion{
			FinalHead: completion.FinalHead, TuringReportPath: completion.TuringReportPath,
			Verification: cloneStrings(completion.Verification), RemoteArtifactURL: completion.RemoteArtifactURL, CompletedAt: completion.CompletedAt,
		}
	}
	if execution.Failure != nil {
		failure := execution.Failure
		result.Failure = &model.ExecutionFailure{OperationID: failure.OperationID, Code: failure.Code, Message: failure.Message, At: failure.At}
	}
	if execution.SyncBaseEvents != nil {
		result.SyncBaseEvents = make([]model.ExecutionSyncBaseEvent, len(execution.SyncBaseEvents))
		for index, event := range execution.SyncBaseEvents {
			result.SyncBaseEvents[index] = model.ExecutionSyncBaseEvent{
				Mode: event.Mode, BaseBranch: event.BaseBranch, BaseOID: event.BaseOID, MergeCommit: event.MergeCommit,
				ConflictFiles: event.ConflictFiles, Actor: event.Actor, At: event.At,
			}
		}
	}
	return result
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func publicError(err error) error {
	if errors.Is(err, leasecontract.ErrMalformedSchema) {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			return syntax
		}
	}
	var unsupported leasecontract.UnsupportedSchemaError
	if errors.As(err, &unsupported) {
		return unsupported
	}
	return err
}
