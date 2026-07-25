package issueops

import (
	"context"
	"fmt"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

const (
	ExecutionActionPrepare   = "prepare"
	ExecutionActionStatus    = "status"
	ExecutionActionClaim     = "claim"
	ExecutionActionRelease   = "release"
	ExecutionActionReplace   = "replace"
	ExecutionActionReconcile = "reconcile"
	ExecutionActionComplete  = "complete"
)

type ExecutionActionRequest struct {
	Action                string            `json:"action"`
	ID                    string            `json:"id"`
	Mode                  string            `json:"mode,omitempty"`
	Actor                 model.NativeActor `json:"actor,omitempty"`
	CWD                   string            `json:"cwd,omitempty"`
	OwnerHost             string            `json:"owner_host,omitempty"`
	OwnerModel            string            `json:"owner_model,omitempty"`
	OwnerEffort           string            `json:"owner_effort,omitempty"`
	Generation            uint64            `json:"generation,omitempty"`
	ExpectedGeneration    uint64            `json:"expected_generation,omitempty"`
	TokenFile             string            `json:"claim_token_file,omitempty"`
	IssueBodySHA256       string            `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256   string            `json:"context_packet_sha256,omitempty"`
	ReplaceAction         string            `json:"replace_action,omitempty"`
	InventoryFingerprint  string            `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string            `json:"quiescence_fingerprint,omitempty"`
	Reason                string            `json:"reason,omitempty"`
	Preview               bool              `json:"preview,omitempty"`
	Confirm               bool              `json:"confirm,omitempty"`
	FinalHead             string            `json:"final_head,omitempty"`
	TuringReportPath      string            `json:"turing_report_path,omitempty"`
	Verification          []string          `json:"verification,omitempty"`
	RemoteArtifactURL     string            `json:"remote_artifact_url,omitempty"`
}

type ExecutionActionDependencies struct {
	Direct    port.ExecutionWorkspaceProvisioner
	Orca      port.ExecutionOrcaProvisioner
	OrcaOwner port.ExecutionOrcaOwnerInspector
	ReadIssue ExecutionIssueSnapshotReadFunc
	RemotePR  RemotePullRequestDependencies
}

func ExecuteExecution(ctx context.Context, stateRoot string, req ExecutionActionRequest, deps ExecutionActionDependencies) (any, error) {
	switch req.Action {
	case ExecutionActionPrepare:
		return PrepareExecution(ctx, stateRoot, ExecutionPrepareRequest{
			ID: req.ID, Mode: req.Mode, Actor: req.Actor, CWD: req.CWD,
			OwnerHost: req.OwnerHost, OwnerModel: req.OwnerModel, OwnerEffort: req.OwnerEffort, Confirm: req.Confirm,
		}, ExecutionPrepareDependencies{Direct: deps.Direct, Orca: deps.Orca, ReadIssue: deps.ReadIssue})
	case ExecutionActionStatus:
		return StatusExecution(stateRoot, req.ID)
	case ExecutionActionClaim:
		return ClaimExecutionWithDependencies(ctx, stateRoot, ExecutionClaimRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD, TokenFile: req.TokenFile,
			IssueBodySHA256: req.IssueBodySHA256, ContextPacketSHA256: req.ContextPacketSHA256,
		}, ExecutionClaimDependencies{ReadIssue: deps.ReadIssue})
	case ExecutionActionRelease:
		return ReleaseExecution(stateRoot, ExecutionReleaseRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
		})
	case ExecutionActionReplace:
		return ReplaceExecutionWithDependencies(ctx, stateRoot, ExecutionReplaceRequest{
			ID: req.ID, Action: req.ReplaceAction, ExpectedGeneration: req.ExpectedGeneration,
			InventoryFingerprint: req.InventoryFingerprint, QuiescenceFingerprint: req.QuiescenceFingerprint,
			Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
			// reseed의 재봉인이 현재 이슈 본문을 다시 읽어야 하므로 prepare/claim과
			// 같은 리더를 함께 넘긴다.
		}, ExecutionReplaceDependencies{OrcaOwner: deps.OrcaOwner, ReadIssue: deps.ReadIssue})
	case ExecutionActionReconcile:
		return ReconcileExecutionWithDependencies(ctx, stateRoot, ExecutionReconcileRequest{
			ID: req.ID, Preview: req.Preview, Confirm: req.Confirm, Actor: req.Actor, CWD: req.CWD,
		}, ExecutionReconcileDependencies{Orca: deps.Orca, ReadIssue: deps.ReadIssue, RemotePR: deps.RemotePR})
	case ExecutionActionComplete:
		return CompleteExecution(stateRoot, ExecutionCompleteRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
			FinalHead: req.FinalHead, TuringReportPath: req.TuringReportPath,
			Verification: req.Verification, RemoteArtifactURL: req.RemoteArtifactURL, Confirm: req.Confirm,
		})
	default:
		return nil, fmt.Errorf("unsupported issueops execution action %q", req.Action)
	}
}
