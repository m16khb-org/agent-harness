package issueops

import (
	"context"
	"fmt"
)

func invokeExecutionPrepareHandler(ctx context.Context, stateRoot string, request ExecutionPrepareRequest, invocation ExecutionPrepareInvocation, handler ExecutionPrepareHandler) (ExecutionPrepareResult, error) {
	if handler == nil {
		return ExecutionPrepareResult{ID: request.ID}, ErrPrepareHandlerUnavailable
	}
	return handler(ctx, stateRoot, request, invocation)
}

func ExecuteExecution(ctx context.Context, stateRoot string, req ExecutionActionRequest, deps ExecutionActionDependencies) (any, error) {
	readIssue, snapshotSource, err := executionIssueSnapshotReaderForAction(stateRoot, req, deps.ReadIssue)
	if err != nil {
		return nil, err
	}
	deps.ReadIssue = readIssue
	result, err := executeExecutionAction(ctx, stateRoot, req, deps)
	if err != nil {
		return result, err
	}
	return withExecutionIssueSnapshotSource(result, snapshotSource()), nil
}

func executeExecutionAction(ctx context.Context, stateRoot string, req ExecutionActionRequest, deps ExecutionActionDependencies) (any, error) {
	switch req.Action {
	case ExecutionActionPrepare:
		return invokeExecutionPrepareHandler(ctx, stateRoot, ExecutionPrepareRequest{
			ID: req.ID, Mode: req.Mode, Actor: req.Actor, CWD: req.CWD,
			OwnerHost: req.OwnerHost, OwnerModel: req.OwnerModel, OwnerEffort: req.OwnerEffort,
			IssueSnapshotFile: req.IssueSnapshotFile,
			DirectReason:      req.DirectReason, ExpectedReadinessFingerprint: req.ExpectedReadinessFingerprint, Confirm: req.Confirm,
		}, ExecutionPrepareInvocation{ReadIssue: deps.ReadIssue}, deps.Prepare)
	case ExecutionActionStatus:
		return StatusExecution(stateRoot, req.ID)
	case ExecutionActionClaim:
		if deps.Claim == nil {
			return ExecutionResult{OK: false, ID: req.ID}, ErrClaimHandlerUnavailable
		}
		return deps.Claim(ctx, stateRoot, ExecutionClaimRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD, TokenFile: req.TokenFile, ClaimCurrentToken: req.ClaimCurrentToken,
			IssueBodySHA256: req.IssueBodySHA256, ContextPacketSHA256: req.ContextPacketSHA256,
		}, ExecutionClaimDependencies{ReadIssue: deps.ReadIssue})
	case ExecutionActionRelease:
		if deps.Release == nil {
			return ExecutionResult{OK: false, ID: req.ID}, ErrReleaseHandlerUnavailable
		}
		return deps.Release(ctx, stateRoot, ExecutionReleaseRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
		})
	case ExecutionActionReplace:
		if req.ReplaceAction == ExecutionReplaceReseed {
			if deps.Reseed == nil {
				return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.ReplaceAction}, ErrReseedHandlerUnavailable
			}
			return deps.Reseed(ctx, stateRoot, ExecutionReseedRequest{
				ID: req.ID, ExpectedGeneration: req.ExpectedGeneration, CompletionGeneration: req.CompletionGeneration, InventoryFingerprint: req.InventoryFingerprint,
				Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
				ReadIssue: deps.ReadIssue,
			})
		}
		return ReplaceExecutionWithDependencies(ctx, stateRoot, ExecutionReplaceRequest{
			ID: req.ID, Action: req.ReplaceAction, ExpectedGeneration: req.ExpectedGeneration, CompletionGeneration: req.CompletionGeneration,
			InventoryFingerprint: req.InventoryFingerprint, QuiescenceFingerprint: req.QuiescenceFingerprint,
			Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
			// finalize/reseed 재봉인이 현재 이슈 본문을 다시 읽어야 하므로
			// prepare/claim과 같은 리더를 함께 넘긴다.
		}, ExecutionReplaceDependencies{OrcaOwner: deps.OrcaOwner, BaseSync: deps.BaseSync, ReadIssue: deps.ReadIssue})
	case ExecutionActionResume:
		if !req.Confirm {
			return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires confirm")
		}
		if deps.Resume == nil {
			return ExecutionResumeResult{OK: false, ID: req.ID}, ErrResumeHandlerUnavailable
		}
		return deps.Resume(ctx, stateRoot, ExecutionResumeRequest{
			ID: req.ID, ExpectedGeneration: req.ExpectedGeneration,
			Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
		})
	case ExecutionActionReconcile:
		return ReconcileExecutionWithDependencies(ctx, stateRoot, ExecutionReconcileRequest{
			ID: req.ID, Preview: req.Preview, Confirm: req.Confirm, Actor: req.Actor, CWD: req.CWD,
		}, ExecutionReconcileDependencies{
			Orca: deps.Orca, ReadIssue: deps.ReadIssue,
			Handler: deps.Reconcile, RemoteReconcile: deps.RemoteReconcile,
		})
	case ExecutionActionComplete:
		if deps.Complete == nil {
			return ExecutionResult{OK: false, ID: req.ID}, ErrCompleteHandlerUnavailable
		}
		return deps.Complete(ctx, stateRoot, ExecutionCompleteRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
			FinalHead: req.FinalHead, TuringReportPath: req.TuringReportPath,
			Verification: req.Verification, RemoteArtifactURL: req.RemoteArtifactURL, Confirm: req.Confirm,
		})
	default:
		return nil, fmt.Errorf("unsupported issueops execution action %q", req.Action)
	}
}
