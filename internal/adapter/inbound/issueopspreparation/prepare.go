// Package issueopspreparation maps execution requests to the preparation
// application service.
package issueopspreparation

import (
	"context"
	"errors"

	issueopscontract "agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/port"
)

type service interface {
	Prepare(context.Context, preparationcontract.Command) (preparationcontract.Result, error)
}

type Handler struct{ service service }

// 반환 타입은 어댑터의 이름 붙은 핸들러 타입 대신 같은 시그니처를 직접 쓴다.
// Go에서 두 형태는 할당 호환이므로 소비자는 그대로 동작하고, inbound 어댑터는
// issueops 어댑터를 알 필요가 없어진다.
func NewHandler(service service) func(context.Context, string, issueopscontract.ExecutionPrepareRequest, port.ExecutionPrepareInvocation) (issueopscontract.ExecutionPrepareResult, error) {
	return Handler{service: service}.Handle
}

func (handler Handler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionPrepareRequest, _ port.ExecutionPrepareInvocation) (issueopscontract.ExecutionPrepareResult, error) {
	if handler.service == nil {
		return issueopscontract.ExecutionPrepareResult{ID: request.ID}, issueopscontract.ErrPrepareHandlerUnavailable
	}
	result, err := handler.service.Prepare(ctx, preparationcontract.Command{
		ID: request.ID, Mode: request.Mode, Actor: preparationActor(request.Actor), CWD: request.CWD,
		OwnerHost: request.OwnerHost, OwnerModel: request.OwnerModel, OwnerEffort: request.OwnerEffort,
		IssueSnapshotFile: request.IssueSnapshotFile,
		DirectReason:      request.DirectReason, ExpectedReadinessFingerprint: request.ExpectedReadinessFingerprint, Confirm: request.Confirm,
	})
	return coreResult(result), publicError(err)
}

func preparationActor(actor issueopscontract.NativeActor) preparationcontract.Actor {
	result := preparationcontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	return result
}

func coreResult(result preparationcontract.Result) issueopscontract.ExecutionPrepareResult {
	return issueopscontract.ExecutionPrepareResult{
		OK: result.OK, ID: result.ID, Preview: result.Preview,
		RequestedMode: result.RequestedMode, ResolvedMode: result.ResolvedMode, FallbackCode: result.FallbackCode,
		ProbeAttempted: result.ProbeAttempted, ProbeAvailable: result.ProbeAvailable, ProbeReady: result.ProbeReady,
		ProbeCode: result.ProbeCode, ReadinessFingerprint: result.ReadinessFingerprint, ExplicitDirectReason: result.ExplicitDirectReason,
		Workspace: coreWorkspace(result.Workspace), Execution: coreExecution(result.Execution),
		ClaimTokenPath: result.ClaimTokenPath, IssueBodySHA256: result.IssueBodySHA256,
		ContextPacketPath: result.ContextPacketPath, ContextPacketSHA256: result.ContextPacketSHA256,
		OwnerPromptPath: result.OwnerPromptPath, OwnerPromptSHA256: result.OwnerPromptSHA256,
		IssueSnapshotSource: result.IssueSnapshotSource, NextCommand: result.NextCommand,
	}
}

func coreWorkspace(workspace preparationcontract.Workspace) issueopscontract.Workspace {
	return issueopscontract.Workspace{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
		BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree, Driver: workspace.Driver, LinkedAt: workspace.LinkedAt,
	}
}

func coreExecution(execution *leasecontract.Execution) *issueopscontract.Execution {
	if execution == nil {
		return nil
	}
	result := &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionMode(execution.Mode), Workspace: coreWorkspace(execution.Workspace),
		Lease: issueopscontract.WriteLease{
			Generation: execution.Lease.Generation, Status: issueopscontract.LeaseStatus(execution.Lease.Status),
			ClaimTokenSHA256: execution.Lease.ClaimTokenSHA256, ClaimedAt: execution.Lease.ClaimedAt,
			ReleasedAt: execution.Lease.ReleasedAt, ReplacedAt: execution.Lease.ReplacedAt, ReplacementReason: execution.Lease.ReplacementReason,
		},
	}
	if execution.Selection != nil {
		selection := execution.Selection
		result.Selection = &issueopscontract.ExecutionSelection{
			RequestedMode: selection.RequestedMode, ResolvedMode: selection.ResolvedMode,
			ProbeAttempted: selection.ProbeAttempted, ProbeAvailable: selection.ProbeAvailable, ProbeReady: selection.ProbeReady,
			ProbeCode: selection.ProbeCode, FallbackCode: selection.FallbackCode,
			ReadinessFingerprint: selection.ReadinessFingerprint, SelectedAt: selection.SelectedAt,
			ExplicitDirectReason: selection.ExplicitDirectReason,
		}
	}
	if execution.Lease.Holder != nil {
		holder := execution.Lease.Holder
		result.Lease.Holder = &issueopscontract.NativeActor{Host: holder.Host, SessionID: holder.SessionID, AgentID: holder.AgentID}
		if holder.SessionProcess != nil {
			result.Lease.Holder.SessionProcess = &issueopscontract.NativeProcessReceipt{
				PID: holder.SessionProcess.PID, StartedAt: holder.SessionProcess.StartedAt, Executable: holder.SessionProcess.Executable,
			}
		}
	}
	if execution.Orca != nil {
		binding := execution.Orca
		result.Orca = &issueopscontract.OrcaBinding{
			RuntimeID: binding.RuntimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
			RunID: binding.RunID, WorktreeInstanceID: binding.WorktreeInstanceID, LeaseGeneration: binding.LeaseGeneration,
			ArtifactIdentityVersion: binding.ArtifactIdentityVersion,
			IssueBodySHA256:         binding.IssueBodySHA256, ContextPacketSHA256: binding.ContextPacketSHA256, OwnerPromptSHA256: binding.OwnerPromptSHA256,
			OwnerHost: binding.OwnerHost, OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
			TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		}
	}
	if execution.Pending != nil {
		pending := execution.Pending
		result.Pending = &issueopscontract.ExternalIntent{OperationID: pending.OperationID, Kind: pending.Kind, Marker: pending.Marker, StartedAt: pending.StartedAt}
	}
	if execution.Completion != nil {
		completion := execution.Completion
		result.Completion = &issueopscontract.ExecutionCompletion{
			Generation: completion.Generation, FinalHead: completion.FinalHead, TuringReportPath: completion.TuringReportPath,
			Verification: cloneStrings(completion.Verification), RemoteArtifactURL: completion.RemoteArtifactURL, CompletedAt: completion.CompletedAt,
		}
	}
	if execution.CompletionHistory != nil {
		result.CompletionHistory = make([]issueopscontract.ExecutionCompletionHistory, len(execution.CompletionHistory))
		for index, entry := range execution.CompletionHistory {
			result.CompletionHistory[index] = issueopscontract.ExecutionCompletionHistory{
				Generation: entry.Generation,
				Completion: issueopscontract.ExecutionCompletion{Generation: entry.Completion.Generation, FinalHead: entry.Completion.FinalHead, TuringReportPath: entry.Completion.TuringReportPath, Verification: cloneStrings(entry.Completion.Verification), RemoteArtifactURL: entry.Completion.RemoteArtifactURL, CompletedAt: entry.Completion.CompletedAt},
				Reason:     entry.Reason,
				ReopenedAt: entry.ReopenedAt,
			}
		}
	}
	if execution.Failure != nil {
		failure := execution.Failure
		result.Failure = &issueopscontract.ExecutionFailure{OperationID: failure.OperationID, Code: failure.Code, Message: failure.Message, At: failure.At}
	}
	if execution.SyncBaseEvents != nil {
		result.SyncBaseEvents = make([]issueopscontract.ExecutionSyncBaseEvent, len(execution.SyncBaseEvents))
		for index, event := range execution.SyncBaseEvents {
			result.SyncBaseEvents[index] = issueopscontract.ExecutionSyncBaseEvent{
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
	if errors.Is(err, statecontract.ErrInvalidState) {
		return statecontract.ErrInvalidState
	}
	return err
}
