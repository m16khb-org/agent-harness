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

type ExecutionActionRequestV1 struct {
	Action                string              `json:"action"`
	ID                    string              `json:"id"`
	Mode                  string              `json:"mode,omitempty"`
	Actor                 model.NativeActorV1 `json:"actor,omitempty"`
	CWD                   string              `json:"cwd,omitempty"`
	OwnerHost             string              `json:"owner_host,omitempty"`
	OwnerModel            string              `json:"owner_model,omitempty"`
	OwnerEffort           string              `json:"owner_effort,omitempty"`
	Generation            uint64              `json:"generation,omitempty"`
	ExpectedGeneration    uint64              `json:"expected_generation,omitempty"`
	TokenFile             string              `json:"claim_token_file,omitempty"`
	IssueBodySHA256       string              `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256   string              `json:"context_packet_sha256,omitempty"`
	ReplaceAction         string              `json:"replace_action,omitempty"`
	InventoryFingerprint  string              `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string              `json:"quiescence_fingerprint,omitempty"`
	Reason                string              `json:"reason,omitempty"`
	Preview               bool                `json:"preview,omitempty"`
	Confirm               bool                `json:"confirm,omitempty"`
	FinalHead             string              `json:"final_head,omitempty"`
	TuringReportPath      string              `json:"turing_report_path,omitempty"`
	Verification          []string            `json:"verification,omitempty"`
	RemoteArtifactURL     string              `json:"remote_artifact_url,omitempty"`
}

type ExecutionActionDependenciesV1 struct {
	Direct    port.ExecutionWorkspaceProvisioner
	Orca      port.ExecutionOrcaProvisioner
	OrcaOwner port.ExecutionOrcaOwnerInspector
	ReadIssue ExecutionIssueSnapshotReadFuncV1
	RemotePR  RemotePullRequestDependenciesV1
}

func ExecuteExecutionV1(ctx context.Context, stateRoot string, req ExecutionActionRequestV1, deps ExecutionActionDependenciesV1) (any, error) {
	switch req.Action {
	case ExecutionActionPrepare:
		return PrepareExecutionV1(ctx, stateRoot, ExecutionPrepareRequestV1{
			ID: req.ID, Mode: req.Mode, Actor: req.Actor, CWD: req.CWD,
			OwnerHost: req.OwnerHost, OwnerModel: req.OwnerModel, OwnerEffort: req.OwnerEffort, Confirm: req.Confirm,
		}, ExecutionPrepareDependenciesV1{Direct: deps.Direct, Orca: deps.Orca, ReadIssue: deps.ReadIssue})
	case ExecutionActionStatus:
		return StatusExecutionV1(stateRoot, req.ID)
	case ExecutionActionClaim:
		return ClaimExecutionV1WithDependencies(ctx, stateRoot, ExecutionClaimRequestV1{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD, TokenFile: req.TokenFile,
			IssueBodySHA256: req.IssueBodySHA256, ContextPacketSHA256: req.ContextPacketSHA256,
		}, ExecutionClaimDependenciesV1{ReadIssue: deps.ReadIssue})
	case ExecutionActionRelease:
		return ReleaseExecutionV1(stateRoot, ExecutionReleaseRequestV1{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
		})
	case ExecutionActionReplace:
		return ReplaceExecutionV1WithDependencies(ctx, stateRoot, ExecutionReplaceRequestV1{
			ID: req.ID, Action: req.ReplaceAction, ExpectedGeneration: req.ExpectedGeneration,
			InventoryFingerprint: req.InventoryFingerprint, QuiescenceFingerprint: req.QuiescenceFingerprint,
			Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
		}, ExecutionReplaceDependenciesV1{OrcaOwner: deps.OrcaOwner})
	case ExecutionActionReconcile:
		return ReconcileExecutionV1WithDependencies(ctx, stateRoot, ExecutionReconcileRequestV1{
			ID: req.ID, Preview: req.Preview, Confirm: req.Confirm, Actor: req.Actor, CWD: req.CWD,
		}, ExecutionReconcileDependenciesV1{Orca: deps.Orca, ReadIssue: deps.ReadIssue, RemotePR: deps.RemotePR})
	case ExecutionActionComplete:
		return CompleteExecutionV1(stateRoot, ExecutionCompleteRequestV1{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
			FinalHead: req.FinalHead, TuringReportPath: req.TuringReportPath,
			Verification: req.Verification, RemoteArtifactURL: req.RemoteArtifactURL, Confirm: req.Confirm,
		})
	default:
		return nil, fmt.Errorf("unsupported issueops execution action %q", req.Action)
	}
}
