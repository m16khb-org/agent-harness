package issueops

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-harness/internal/contract/issueops"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/port"
)

// ResolveExecutionPreparationWorkspace exposes the predecessor's canonical
// workspace calculation as a granular composition effect.
func ResolveExecutionPreparationWorkspace(snapshot preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.WorkspaceRequest{}, err
	}
	request, err := executionWorkspaceRequest(record, confirm)
	if err != nil {
		return preparationcontract.WorkspaceRequest{}, err
	}
	return preparationcontract.WorkspaceRequest{
		LifecycleID: request.LifecycleID, SourceRoot: request.SourceRoot, Root: request.Root,
		Branch: request.Branch, BaseBranch: request.BaseBranch, BaseHead: request.BaseHead,
		ParentWorktree: request.ParentWorktree, Confirm: request.Confirm,
	}, nil
}

// ValidateExecutionPreparationOrcaProbe keeps branch authority validation on
// the current durable record immediately before external preparation.
func ValidateExecutionPreparationOrcaProbe(stateRoot, id string, request preparationcontract.ProbeRequest) (string, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return "orca_branch_precheck_failed", err
	}
	if err := ensureOrcaBranchIsFree(record, request.Workspace.Branch); err != nil {
		return "orca_branch_name_taken", err
	}
	return "", nil
}

func ReadExecutionPreparationOwnerEvidence(ctx context.Context, snapshot preparationcontract.Snapshot, readIssue ExecutionIssueSnapshotReadFunc) (preparationcontract.OwnerEvidence, error) {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	owner, err := readExecutionOwnerSnapshot(ctx, record, readIssue)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	identity, err := (preparationcontract.IntentCodec{}).PrepareIssueIdentity(snapshot.Record)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	return preparationcontract.OwnerEvidence{
		IssueURL: owner.issue.URL, IssueBody: owner.issue.Body, BodySHA256: owner.issue.BodySHA256,
		Provider: identity.Provider, Issue: identity.Issue,
	}, nil
}

func MaterializeExecutionPreparationDirect(stateRoot string, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return err
	}
	record.WorktreePath = receipt.Root
	record.Execution = &issueops.Execution{
		Mode: issueops.ExecutionModeDirect,
		Workspace: issueops.Workspace{
			SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch,
			BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree, Driver: receipt.Driver,
		},
	}
	_, err = materializeStagedArtifacts(stateRoot, record)
	return err
}

func PrepareExecutionPreparationOwner(
	ctx context.Context,
	stateRoot string,
	snapshot preparationcontract.Snapshot,
	command preparationcontract.Command,
	intent preparationcontract.Intent,
	receipt preparationcontract.IntentReceipt,
	readIssue ExecutionIssueSnapshotReadFunc,
) (preparationcontract.OwnerArtifacts, error) {
	if receipt.Workspace == nil {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
	}
	workspaceReceipt := port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: receipt.Workspace.Workspace.SourceRoot, Root: receipt.Workspace.Workspace.Root,
			Branch: receipt.Workspace.Workspace.Branch, BaseHead: receipt.Workspace.Workspace.BaseHead,
			ParentWorktree: receipt.Workspace.Workspace.ParentWorktree,
			Driver:         receipt.Workspace.Workspace.Driver, Exists: receipt.Workspace.Workspace.Exists,
		},
		RuntimeID: receipt.Workspace.RuntimeID, RepoID: receipt.Workspace.RepoID,
		WorktreeID: receipt.Workspace.WorktreeID, WorktreeInstanceID: receipt.Workspace.WorktreeInstanceID,
	}
	workspaceRequest := port.ExecutionWorkspaceRequest{
		LifecycleID: intent.Workspace.LifecycleID, SourceRoot: intent.Workspace.SourceRoot, Root: intent.Workspace.Root,
		Branch: intent.Workspace.Branch, BaseBranch: intent.Workspace.BaseBranch, BaseHead: intent.Workspace.BaseHead,
		ParentWorktree: intent.Workspace.ParentWorktree, Confirm: intent.Workspace.Confirm,
	}
	if validateExecutionOrcaWorkspaceReceipt(workspaceRequest, workspaceReceipt) != nil {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
	}
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	record.WorktreePath = workspaceReceipt.Workspace.Root
	record.Execution.Workspace = workspaceFromReceipt(workspaceReceipt.Workspace, intent.StartedAt)
	owner, err := readExecutionOwnerSnapshot(ctx, record, readIssue)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	if owner.issue.BodySHA256 != intent.IssueBodySHA256 {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("remote issue body drifted before owner launch recovery")
	}
	tokenSHA256, err := createOrAdoptClaimToken(record)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	manifest, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	artifacts, err := buildExecutionOwnerArtifacts(record, ExecutionPrepareRequest{
		ID: command.ID, Mode: preparationcontract.ModeOrca,
		OwnerHost: command.OwnerHost, OwnerModel: command.OwnerModel, OwnerEffort: command.OwnerEffort,
	}, owner, manifest)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	return preparationcontract.OwnerArtifacts{
		ClaimTokenPath: claimTokenPath(record), ClaimTokenSHA256: tokenSHA256,
		ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		OwnerPromptPath: artifacts.promptPath, OwnerPromptSHA256: artifacts.promptSHA256,
	}, nil
}

func HydrateExecutionPreparationLaunch(stateRoot, id string, request preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	if record.Execution == nil || record.Execution.Pending == nil {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed Orca intent is unavailable")
	}
	operationID := record.Execution.Pending.OperationID
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	raw, ok, err := database.Get(externalIntentBucket, operationID)
	if err != nil || !ok {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed Orca intent is unavailable")
	}
	intent, err := preparationIntentCodec.Decode(operationID, raw)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	if request.Stage != intent.Stage || request.Marker != intent.Marker || request.Launch == nil {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed owner launch identity changed")
	}
	hydrated, err := executionOrcaIntentRequest(record, intent)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	if hydrated.Launch == nil || request.Launch.PromptPath != hydrated.Launch.PromptPath ||
		request.Launch.PromptSHA256 != hydrated.Launch.PromptSHA256 ||
		request.Launch.ContextPacketPath != hydrated.Launch.ContextPacketPath ||
		request.Launch.ContextPacketSHA256 != hydrated.Launch.ContextPacketSHA256 {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed owner launch identity changed")
	}
	request.Launch.Prompt = hydrated.Launch.Prompt
	return request, nil
}

func NewExecutionPreparationOperationID() (string, error) { return newExecutionOperationID() }

func executionPreparationCoreRecord(snapshot preparationcontract.Snapshot) (issueops.IssueOpsRecord, error) {
	if len(snapshot.RecordRaw) == 0 {
		return issueops.IssueOpsRecord{}, fmt.Errorf("preparation raw record snapshot is required")
	}
	var record issueops.IssueOpsRecord
	if err := json.Unmarshal(snapshot.RecordRaw, &record); err != nil {
		return issueops.IssueOpsRecord{}, err
	}
	return record, nil
}
