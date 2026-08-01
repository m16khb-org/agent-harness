package issueops

import (
	"context"
	"errors"
	"fmt"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

var (
	ErrPrepareHandlerUnavailable                    = errors.New("issueops execution prepare handler is not configured")
	ErrClaimHandlerUnavailable                      = errors.New("issueops execution claim handler is not configured")
	ErrReleaseHandlerUnavailable                    = errors.New("issueops execution release handler is not configured")
	ErrReseedHandlerUnavailable                     = errors.New("issueops execution reseed handler is not configured")
	ErrResumeHandlerUnavailable                     = errors.New("issueops execution resume handler is not configured")
	ErrReconcileHandlerUnavailable                  = errors.New("issueops execution reconcile handler is not configured")
	ErrCompleteHandlerUnavailable                   = errors.New("issueops execution complete handler is not configured")
	ErrRemotePullRequestCreateHandlerUnavailable    = errors.New("remote pull request provider is unavailable")
	ErrRemotePullRequestReconcileHandlerUnavailable = errors.New("remote reconcile provider is unavailable")
)

type ExecutionPrepareHandler func(context.Context, string, ExecutionPrepareRequest) (ExecutionPrepareResult, error)
type ExecutionClaimHandler func(context.Context, string, ExecutionClaimRequest, ExecutionClaimDependencies) (ExecutionResult, error)
type ExecutionReleaseHandler func(context.Context, string, ExecutionReleaseRequest) (ExecutionResult, error)
type ExecutionReseedHandler func(context.Context, string, ExecutionReseedRequest) (ExecutionReplaceResult, error)
type ExecutionResumeHandler func(context.Context, string, ExecutionResumeRequest) (ExecutionResumeResult, error)
type ExecutionReconcileHandler func(context.Context, string, ExecutionReconcileRequest, ExecutionReconcileDependencies) (ExecutionReconcileResult, error)
type ExecutionCompleteHandler func(context.Context, string, ExecutionCompleteRequest) (ExecutionResult, error)

type RemotePullRequestCreateHandler func(context.Context, string, RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)
type RemotePullRequestReconcileHandler func(context.Context, string, ExecutionReconcileRequest) (ExecutionReconcileResult, error)

type RemotePublicationHandlers struct {
	Create    RemotePullRequestCreateHandler
	Reconcile RemotePullRequestReconcileHandler
}

func invokeExecutionPrepareHandler(ctx context.Context, stateRoot string, request ExecutionPrepareRequest, handler ExecutionPrepareHandler) (ExecutionPrepareResult, error) {
	if handler == nil {
		return ExecutionPrepareResult{ID: request.ID}, ErrPrepareHandlerUnavailable
	}
	return handler(ctx, stateRoot, request)
}

const (
	ExecutionActionPrepare   = "prepare"
	ExecutionActionStatus    = "status"
	ExecutionActionClaim     = "claim"
	ExecutionActionRelease   = "release"
	ExecutionActionReplace   = "replace"
	ExecutionActionResume    = "resume"
	ExecutionActionReconcile = "reconcile"
	ExecutionActionComplete  = "complete"
)

type ExecutionActionRequest struct {
	Action                string                               `json:"action"`
	ID                    string                               `json:"id"`
	Mode                  string                               `json:"mode,omitempty"`
	Actor                 model.NativeActor                    `json:"actor,omitempty"`
	CWD                   string                               `json:"cwd,omitempty"`
	OwnerHost             string                               `json:"owner_host,omitempty"`
	OwnerModel            string                               `json:"owner_model,omitempty"`
	OwnerEffort           string                               `json:"owner_effort,omitempty"`
	Generation            uint64                               `json:"generation,omitempty"`
	ExpectedGeneration    uint64                               `json:"expected_generation,omitempty"`
	TokenFile             string                               `json:"claim_token_file,omitempty"`
	IssueBodySHA256       string                               `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256   string                               `json:"context_packet_sha256,omitempty"`
	ReplaceAction         string                               `json:"replace_action,omitempty"`
	InventoryFingerprint  string                               `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string                               `json:"quiescence_fingerprint,omitempty"`
	Reason                string                               `json:"reason,omitempty"`
	Preview               bool                                 `json:"preview,omitempty"`
	Confirm               bool                                 `json:"confirm,omitempty"`
	FinalHead             string                               `json:"final_head,omitempty"`
	TuringReportPath      string                               `json:"turing_report_path,omitempty"`
	Verification          []string                             `json:"verification,omitempty"`
	RemoteArtifactURL     string                               `json:"remote_artifact_url,omitempty"`
	IssueSnapshot         *port.ExecutionIssueSnapshotEvidence `json:"issue_snapshot,omitempty"`
}

type ExecutionActionDependencies struct {
	Prepare   ExecutionPrepareHandler
	Orca      port.ExecutionOrcaProvisioner
	OrcaOwner port.ExecutionOrcaOwnerInspector
	ReadIssue ExecutionIssueSnapshotReadFunc
	Claim     ExecutionClaimHandler
	Release   ExecutionReleaseHandler
	Reseed    ExecutionReseedHandler
	Resume    ExecutionResumeHandler
	Reconcile ExecutionReconcileHandler
	Complete  ExecutionCompleteHandler
	// RemoteReconcile handles remote_pr_create recovery independently of the
	// Orca-specific Reconcile handler.
	RemoteReconcile RemotePullRequestReconcileHandler
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
			OwnerHost: req.OwnerHost, OwnerModel: req.OwnerModel, OwnerEffort: req.OwnerEffort, Confirm: req.Confirm,
		}, deps.Prepare)
	case ExecutionActionStatus:
		return StatusExecution(stateRoot, req.ID)
	case ExecutionActionClaim:
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionResult{OK: false, ID: req.ID}, err
		}
		if deps.Claim == nil {
			return ExecutionResult{OK: false, ID: req.ID}, ErrClaimHandlerUnavailable
		}
		return deps.Claim(ctx, stateRoot, ExecutionClaimRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD, TokenFile: req.TokenFile,
			IssueBodySHA256: req.IssueBodySHA256, ContextPacketSHA256: req.ContextPacketSHA256,
		}, ExecutionClaimDependencies{ReadIssue: deps.ReadIssue})
	case ExecutionActionRelease:
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionResult{OK: false, ID: req.ID}, err
		}
		if deps.Release == nil {
			return ExecutionResult{OK: false, ID: req.ID}, ErrReleaseHandlerUnavailable
		}
		return deps.Release(ctx, stateRoot, ExecutionReleaseRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
		})
	case ExecutionActionReplace:
		if req.ReplaceAction == ExecutionReplaceReseed {
			if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
				return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.ReplaceAction}, err
			}
			if deps.Reseed == nil {
				return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.ReplaceAction}, ErrReseedHandlerUnavailable
			}
			return deps.Reseed(ctx, stateRoot, ExecutionReseedRequest{
				ID: req.ID, ExpectedGeneration: req.ExpectedGeneration, InventoryFingerprint: req.InventoryFingerprint,
				Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
				ReadIssue: deps.ReadIssue,
			})
		}
		return ReplaceExecutionWithDependencies(ctx, stateRoot, ExecutionReplaceRequest{
			ID: req.ID, Action: req.ReplaceAction, ExpectedGeneration: req.ExpectedGeneration,
			InventoryFingerprint: req.InventoryFingerprint, QuiescenceFingerprint: req.QuiescenceFingerprint,
			Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
			// finalize/reseed 재봉인이 현재 이슈 본문을 다시 읽어야 하므로
			// prepare/claim과 같은 리더를 함께 넘긴다.
		}, ExecutionReplaceDependencies{OrcaOwner: deps.OrcaOwner, ReadIssue: deps.ReadIssue})
	case ExecutionActionResume:
		if !req.Confirm {
			return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires confirm")
		}
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionResumeResult{OK: false, ID: req.ID}, err
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
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionResult{OK: false, ID: req.ID}, err
		}
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
