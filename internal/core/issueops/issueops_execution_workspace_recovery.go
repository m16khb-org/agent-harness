package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type IssueOpsExecutionWorkspaceReconcileRequest struct {
	ID             string        `json:"id"`
	Actor          IssueOpsActor `json:"actor"`
	WorkspaceEpoch string        `json:"workspace_epoch"`
}

type IssueOpsExecutionWorkspaceRecoveryClient interface {
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
}

type issueOpsExecutionWorkspaceBranchCanonicalizer interface {
	CanonicalizeWorktreeBranch(context.Context, port.OrcaWorktree, string, string) (port.OrcaWorktree, error)
}

// markExecutionWorkspaceRecovery records uncertainty from an Orca worktree
// operation without creating an ownership handoff. Recovery remains an
// explicit preparation concern until a human or the sealed preparer reconciles
// the exact workspace journal.
func markExecutionWorkspaceRecovery(stateRoot, id, epoch, code, message, now string) error {
	return withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		workspace := currentIssueOpsWorkspace(record)
		if workspace == nil {
			return fmt.Errorf("execution workspace is required for workspace recovery")
		}
		if workspace.WorkspaceEpoch != epoch {
			return fmt.Errorf("stale workspace recovery result")
		}
		if workspace.State == handoff.StateRecoveryRequired {
			return nil
		}
		workspace.State = handoff.StateRecoveryRequired
		workspace.Failure = &model.IssueOpsExecutionHandoffFailure{Code: code, Message: message, At: now}
		workspace.UpdatedAt = now
		record.UpdatedAt = now
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

// ReconcileIssueOpsExecutionWorkspaceWorktree adopts exactly one worktree that
// was created after the journal baseline with this cycle's exact marker.
func ReconcileIssueOpsExecutionWorkspaceWorktree(pending model.IssueOpsExecutionWorkspacePendingOperation, id, epoch string, rows []port.OrcaWorktree) (port.OrcaWorktree, error) {
	if pending.Kind != handoff.OperationWorktreeCreate {
		return port.OrcaWorktree{}, fmt.Errorf("pending operation is not worktree_create")
	}
	baseline := make(map[string]struct{}, len(pending.BaselineWorktreeIDs))
	for _, value := range pending.BaselineWorktreeIDs {
		baseline[strings.TrimSpace(value)] = struct{}{}
	}
	marker := issueOpsHandoffMarker(id, epoch, 1)
	candidates := make([]port.OrcaWorktree, 0, 1)
	for _, row := range rows {
		if _, existed := baseline[strings.TrimSpace(row.ID)]; existed || row.Comment != marker {
			continue
		}
		candidates = append(candidates, row)
	}
	if len(candidates) != 1 {
		return port.OrcaWorktree{}, fmt.Errorf("workspace recovery requires exactly one marker candidate; found %d", len(candidates))
	}
	return candidates[0], nil
}

// ReconcileIssueOpsExecutionWorkspace is deliberately workspace-only: it may
// make a verified worktree ready, but cannot create an ownership handoff or
// dispatch any owner resources.
func ReconcileIssueOpsExecutionWorkspace(ctx context.Context, stateRoot string, req IssueOpsExecutionWorkspaceReconcileRequest, client IssueOpsExecutionWorkspaceRecoveryClient, now string) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	workspace := currentIssueOpsWorkspace(record)
	if currentIssueOpsHandoff(record) != nil || workspace == nil || workspace.State != handoff.StateRecoveryRequired || workspace.PendingOperation == nil {
		return IssueOpsRecord{}, fmt.Errorf("workspace reconciliation requires a recovery-required workspace journal without an ownership handoff")
	}
	if strings.TrimSpace(req.WorkspaceEpoch) != workspace.WorkspaceEpoch {
		return IssueOpsRecord{}, fmt.Errorf("workspace reconciliation requires the exact workspace epoch")
	}
	session, err := validateWorkspacePreparationActor(record, workspace.Agent, req.Actor)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if workspace.PreparationSession == nil || *workspace.PreparationSession != session {
		return IssueOpsRecord{}, fmt.Errorf("workspace reconciliation requires the sealed preparation session")
	}
	if client == nil {
		return IssueOpsRecord{}, fmt.Errorf("Orca workspace recovery dependency is unavailable")
	}
	rows, err := client.ListWorktrees(ctx, record.Repo)
	if err != nil {
		return IssueOpsRecord{}, fmt.Errorf("list Orca worktrees for workspace recovery: %w", err)
	}
	candidate, err := ReconcileIssueOpsExecutionWorkspaceWorktree(*workspace.PendingOperation, record.ID, workspace.WorkspaceEpoch, rows)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if strings.TrimPrefix(strings.TrimSpace(candidate.Branch), "refs/heads/") != strings.TrimSpace(record.Branch) {
		canonicalizer, ok := client.(issueOpsExecutionWorkspaceBranchCanonicalizer)
		if !ok || workspace.Orca == nil {
			return IssueOpsRecord{}, fmt.Errorf("workspace recovery candidate branch requires canonicalization support")
		}
		candidate, err = canonicalizer.CanonicalizeWorktreeBranch(ctx, candidate, record.Branch, workspace.Orca.BaseRef)
		if err != nil {
			return IssueOpsRecord{}, fmt.Errorf("canonicalize workspace recovery candidate: %w", err)
		}
	}
	if workspace.Orca == nil || validateCreatedHandoffWorktree(record, workspace.WorkerRoot, workspace.Orca.RepoID, workspace.Orca.BaseRef, candidate) != nil || candidate.Comment != issueOpsHandoffMarker(record.ID, workspace.WorkspaceEpoch, 1) {
		return IssueOpsRecord{}, fmt.Errorf("workspace recovery candidate does not match the sealed worktree identity")
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, req.ID)
		if readErr != nil {
			return readErr
		}
		currentWorkspace := currentIssueOpsWorkspace(current)
		if currentIssueOpsHandoff(current) != nil || currentWorkspace == nil || currentWorkspace.State != handoff.StateRecoveryRequired || currentWorkspace.PendingOperation == nil || currentWorkspace.WorkspaceEpoch != req.WorkspaceEpoch || currentWorkspace.PreparationSession == nil || *currentWorkspace.PreparationSession != session {
			return fmt.Errorf("workspace recovery journal changed before result persist")
		}
		currentWorkspace.Orca = &model.IssueOpsOrcaIdentity{RuntimeID: currentWorkspace.Orca.RuntimeID, RepoID: currentWorkspace.Orca.RepoID, BaseRef: currentWorkspace.Orca.BaseRef, ProviderIssueLinkStatus: providerIssueLinkStatus(current, candidate), WorktreeID: candidate.ID, WorktreeInstanceID: candidate.InstanceID, WorktreePath: filepath.Clean(candidate.Path)}
		currentWorkspace.State = "ready"
		currentWorkspace.PendingOperation = nil
		currentWorkspace.Failure = nil
		currentWorkspace.ProvisionedAt = now
		currentWorkspace.UpdatedAt = now
		current.WorktreePath = filepath.Clean(candidate.Path)
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return persisted, err
}
