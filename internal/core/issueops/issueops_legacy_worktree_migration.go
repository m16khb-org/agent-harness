package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

const (
	IssueOpsLegacyWorktreeMigrationStatePrepared    = "prepared"
	IssueOpsLegacyWorktreeMigrationStateGitRemoved  = "git_removed"
	IssueOpsLegacyWorktreeMigrationStateOrcaManaged = "orca_managed"
)

type IssueOpsLegacyWorktreeMigrationRequest struct {
	ID      string `json:"id"`
	Confirm bool   `json:"confirm,omitempty"`
}

type IssueOpsLegacyWorktreeMigrationResult struct {
	OK           bool                  `json:"ok"`
	ID           string                `json:"id"`
	WorktreePath string                `json:"worktree_path"`
	Branch       string                `json:"branch"`
	State        string                `json:"state"`
	Preview      bool                  `json:"preview,omitempty"`
	NextStep     string                `json:"next_step,omitempty"`
	Orca         *IssueOpsOrcaIdentity `json:"orca,omitempty"`
}

// MigrateIssueOpsLegacyWorktree converts only a clean checkout whose local and
// provider-tracking heads equal the recorded branch preparation SHA. Orca has
// no import API, so the Git worktree and local branch are removed only after a
// durable snapshot is recorded; Orca then recreates the same canonical path.
func MigrateIssueOpsLegacyWorktree(ctx context.Context, stateRoot string, req IssueOpsLegacyWorktreeMigrationRequest, client IssueOpsOrcaWorktreeClient, clock IssueOpsHandoffPrepareClock) (IssueOpsLegacyWorktreeMigrationResult, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsLegacyWorktreeMigrationResult{}, err
	}
	base, err := issueOpsLegacyWorktreePrepareResult(record)
	if err != nil {
		return IssueOpsLegacyWorktreeMigrationResult{}, err
	}
	result := IssueOpsLegacyWorktreeMigrationResult{
		OK: true, ID: record.ID, WorktreePath: base.WorktreePath, Branch: record.Branch,
		NextStep: "run issueops worktree prepare --id " + record.ID + " --orchestrator orca --confirm after migration",
	}
	if record.ExecutionHandoff != nil {
		return result, fmt.Errorf("legacy worktree migration requires no active execution handoff")
	}
	if !req.Confirm {
		result.Preview = true
		result.State = migrationState(record.LegacyWorktreeMigration)
		if result.State == "" {
			result.State = "confirmation_required"
		}
		result.NextStep = "rerun with --confirm only after the legacy worktree is clean and remote-equal"
		return result, nil
	}
	if err := validateHandoffPreparePrerequisites(record); err != nil {
		return result, err
	}
	if client == nil {
		return result, fmt.Errorf("orca probe failed: adapter unavailable")
	}
	probe, err := client.Probe(ctx, port.OrcaProbeRequest{Repo: record.Repo, Agent: "codex", Provider: record.BranchPrepare.Provider})
	if err != nil || !probe.Available || !probe.Ready {
		if err != nil {
			return result, fmt.Errorf("orca probe failed: %w", err)
		}
		return result, fmt.Errorf("orca probe failed: %s", strings.TrimSpace(probe.Code))
	}
	providerRef, err := issueOpsOrcaProviderTrackingRef(probe.RepoRemoteName, record.Branch)
	if err != nil {
		return result, err
	}
	if err := validateHandoffProviderRef(record, providerRef); err != nil {
		return result, err
	}
	worktrees, err := client.ListWorktrees(ctx, record.Repo)
	if err != nil {
		return result, fmt.Errorf("list Orca worktrees before legacy migration: %w", err)
	}
	if existing, err := exactExistingHandoffWorktree(record, result.WorktreePath, probe.RepoID, worktrees); err != nil {
		return result, err
	} else if existing != nil {
		result.State = IssueOpsLegacyWorktreeMigrationStateOrcaManaged
		result.Orca = migrationOrcaIdentity(probe, providerRef, *existing)
		return result, nil
	}
	now := issueOpsHandoffNow(clock)
	migration, err := beginOrResumeLegacyWorktreeMigration(stateRoot, record, result.WorktreePath, providerRef, now)
	if err != nil {
		return result, err
	}
	if migration.State == IssueOpsLegacyWorktreeMigrationStateOrcaManaged {
		result.State = migration.State
		result.Orca = migration.Orca
		return result, nil
	}
	if err := removeLegacyGitWorktree(record, *migration); err != nil {
		return result, err
	}
	migration, err = markLegacyWorktreeGitRemoved(stateRoot, record.ID, *migration, now)
	if err != nil {
		return result, err
	}
	created, err := createLegacyOrcaWorktree(ctx, record, result.WorktreePath, probe, providerRef, client)
	if err != nil {
		return result, err
	}
	if err := validateCreatedHandoffWorktree(record, result.WorktreePath, probe.RepoID, providerRef, created); err != nil {
		return result, err
	}
	if created.Comment != issueOpsLegacyWorktreeMigrationMarker(record.ID) {
		return result, fmt.Errorf("Orca legacy migration response does not contain the exact migration marker")
	}
	listed, err := client.ListWorktrees(ctx, record.Repo)
	if err != nil {
		return result, fmt.Errorf("re-list Orca worktrees after legacy migration: %w", err)
	}
	if !containsExactMigratedWorktree(record, result.WorktreePath, probe.RepoID, providerRef, created, listed) {
		return result, fmt.Errorf("Orca legacy migration result is absent from the runtime worktree inventory")
	}
	migration, err = completeLegacyWorktreeMigration(stateRoot, record.ID, *migration, migrationOrcaIdentity(probe, providerRef, created), now)
	if err != nil {
		return result, err
	}
	result.State = migration.State
	result.Orca = migration.Orca
	return result, nil
}

func issueOpsLegacyWorktreeMigrationMarker(id string) string {
	return "agent-harness:legacy-migration=" + strings.TrimSpace(id)
}

func migrationState(migration *IssueOpsLegacyWorktreeMigration) string {
	if migration == nil {
		return ""
	}
	return strings.TrimSpace(migration.State)
}

func validatePersistedLegacyWorktreeMigration(record IssueOpsRecord) error {
	migration := record.LegacyWorktreeMigration
	if migration == nil {
		return nil
	}
	if record.ExecutionHandoff != nil {
		if migration.State != IssueOpsLegacyWorktreeMigrationStateOrcaManaged {
			return fmt.Errorf("legacy worktree migration cannot coexist with an execution handoff")
		}
		handoffOrca := record.ExecutionHandoff.Orca
		if handoffOrca == nil || migration.Orca == nil || handoffOrca.RuntimeID != migration.Orca.RuntimeID || handoffOrca.RepoID != migration.Orca.RepoID || handoffOrca.BaseRef != migration.Orca.BaseRef {
			return fmt.Errorf("completed legacy worktree migration and execution handoff must reference the same Orca worktree")
		}
		if handoffOrca.WorktreeID == "" && handoffOrca.WorktreeInstanceID == "" && handoffOrca.WorktreePath == "" {
			if record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || record.ExecutionHandoff.PendingOperation == nil || record.ExecutionHandoff.PendingOperation.Kind != handoff.OperationWorktreeCreate || handoffOrca.WorktreeAdopted {
				return fmt.Errorf("completed legacy worktree migration and execution handoff must reference the same Orca worktree")
			}
		} else if !handoffOrca.WorktreeAdopted || handoffOrca.WorktreeID != migration.Orca.WorktreeID || handoffOrca.WorktreeInstanceID != migration.Orca.WorktreeInstanceID || filepath.Clean(handoffOrca.WorktreePath) != migration.WorktreePath {
			return fmt.Errorf("completed legacy worktree migration and execution handoff must reference the same Orca worktree")
		}
	}
	if err := validateLegacyWorktreeMigration(record, *migration, migration.WorktreePath, migration.BaseRef); err != nil {
		return err
	}
	if migration.WorktreePath != filepath.Clean(migration.WorktreePath) || migration.WorktreePath == "." || strings.TrimSpace(migration.Branch) != migration.Branch || strings.TrimSpace(migration.BaseRef) != migration.BaseRef || !validFullCommitSHA(migration.Head) {
		return fmt.Errorf("invalid persisted legacy worktree migration identity")
	}
	if _, err := time.Parse(time.RFC3339Nano, migration.PreparedAt); err != nil {
		return fmt.Errorf("invalid legacy worktree migration prepared timestamp")
	}
	switch migration.State {
	case IssueOpsLegacyWorktreeMigrationStatePrepared:
		if migration.GitRemovedAt != "" || migration.CompletedAt != "" || migration.Orca != nil {
			return fmt.Errorf("prepared legacy worktree migration contains completion evidence")
		}
	case IssueOpsLegacyWorktreeMigrationStateGitRemoved:
		if migration.CompletedAt != "" || migration.Orca != nil {
			return fmt.Errorf("git_removed legacy worktree migration contains completion evidence")
		}
		if _, err := time.Parse(time.RFC3339Nano, migration.GitRemovedAt); err != nil {
			return fmt.Errorf("invalid legacy worktree migration git removal timestamp")
		}
	case IssueOpsLegacyWorktreeMigrationStateOrcaManaged:
		if migration.Orca == nil || strings.TrimSpace(migration.Orca.RuntimeID) == "" || strings.TrimSpace(migration.Orca.RepoID) == "" || strings.TrimSpace(migration.Orca.WorktreeID) == "" || strings.TrimSpace(migration.Orca.WorktreeInstanceID) == "" || filepath.Clean(migration.Orca.WorktreePath) != migration.WorktreePath {
			return fmt.Errorf("orca-managed legacy worktree migration requires exact Orca identity")
		}
		if _, err := time.Parse(time.RFC3339Nano, migration.GitRemovedAt); err != nil {
			return fmt.Errorf("invalid legacy worktree migration git removal timestamp")
		}
		if _, err := time.Parse(time.RFC3339Nano, migration.CompletedAt); err != nil {
			return fmt.Errorf("invalid legacy worktree migration completion timestamp")
		}
	default:
		return fmt.Errorf("unknown legacy worktree migration state")
	}
	return nil
}

func beginOrResumeLegacyWorktreeMigration(stateRoot string, record IssueOpsRecord, worktreePath, baseRef, now string) (*IssueOpsLegacyWorktreeMigration, error) {
	var persisted IssueOpsLegacyWorktreeMigration
	err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return err
		}
		if current.ExecutionHandoff != nil {
			return fmt.Errorf("legacy worktree migration requires no active execution handoff")
		}
		if current.LegacyWorktreeMigration == nil {
			if err := validateRawLegacyWorktreeForMigration(current, worktreePath, baseRef); err != nil {
				return err
			}
			current.LegacyWorktreeMigration = &IssueOpsLegacyWorktreeMigration{
				State: IssueOpsLegacyWorktreeMigrationStatePrepared, WorktreePath: filepath.Clean(worktreePath), Branch: current.Branch,
				Head: current.BranchPrepare.BaseSHA, BaseRef: baseRef, PreparedAt: now,
			}
			current.UpdatedAt = now
			if _, err := writeIssueOps(stateRoot, current); err != nil {
				return err
			}
		}
		migration := current.LegacyWorktreeMigration
		if err := validateLegacyWorktreeMigration(current, *migration, worktreePath, baseRef); err != nil {
			return err
		}
		persisted = *migration
		return nil
	})
	return &persisted, err
}

func validateRawLegacyWorktreeForMigration(record IssueOpsRecord, worktreePath, baseRef string) error {
	if !existingLegacyWorktreeMatches(record, worktreePath) {
		return fmt.Errorf("legacy worktree must exactly match the canonical path, branch, and prepared HEAD")
	}
	if code, status, stderr := preflight.GitCmd(worktreePath, "status", "--porcelain=v1"); code != 0 {
		return fmt.Errorf("inspect legacy worktree status: %s", strings.TrimSpace(stderr))
	} else if strings.TrimSpace(status) != "" {
		return fmt.Errorf("legacy worktree must be clean before migration")
	}
	head := strings.TrimSpace(preflight.GitOut(worktreePath, "rev-parse", "HEAD^{commit}"))
	if head == "" || head != strings.TrimSpace(record.BranchPrepare.BaseSHA) {
		return fmt.Errorf("legacy worktree HEAD does not match the prepared base SHA")
	}
	if code, remoteHead, stderr := preflight.GitCmd(record.Repo, "rev-parse", "--verify", baseRef+"^{commit}"); code != 0 {
		return fmt.Errorf("inspect provider tracking ref before migration: %s", strings.TrimSpace(stderr))
	} else if strings.TrimSpace(remoteHead) != head {
		return fmt.Errorf("legacy worktree must equal the provider tracking ref before migration")
	}
	return nil
}

func validateLegacyWorktreeMigration(record IssueOpsRecord, migration IssueOpsLegacyWorktreeMigration, worktreePath, baseRef string) error {
	if migration.State != IssueOpsLegacyWorktreeMigrationStatePrepared && migration.State != IssueOpsLegacyWorktreeMigrationStateGitRemoved && migration.State != IssueOpsLegacyWorktreeMigrationStateOrcaManaged {
		return fmt.Errorf("unknown legacy worktree migration state")
	}
	if filepath.Clean(migration.WorktreePath) != filepath.Clean(worktreePath) || migration.Branch != record.Branch || migration.Head != record.BranchPrepare.BaseSHA || migration.BaseRef != baseRef {
		return fmt.Errorf("legacy worktree migration snapshot no longer matches the IssueOps record")
	}
	return nil
}

func removeLegacyGitWorktree(record IssueOpsRecord, migration IssueOpsLegacyWorktreeMigration) error {
	if migration.State == IssueOpsLegacyWorktreeMigrationStateOrcaManaged {
		return nil
	}
	worktreePath := filepath.Clean(migration.WorktreePath)
	if _, err := os.Stat(worktreePath); err == nil {
		if err := validateRawLegacyWorktreeForMigration(record, worktreePath, migration.BaseRef); err != nil {
			return err
		}
		if code, _, stderr := preflight.GitCmd(record.Repo, "worktree", "remove", "--", worktreePath); code != 0 {
			return fmt.Errorf("remove clean legacy Git worktree: %s", strings.TrimSpace(stderr))
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect legacy worktree before migration: %w", err)
	}
	ref := "refs/heads/" + record.Branch
	code, _, stderr := preflight.GitCmd(record.Repo, "show-ref", "--verify", "--quiet", ref)
	if code == 0 {
		_, branchHead, _ := preflight.GitCmd(record.Repo, "rev-parse", "--verify", ref+"^{commit}")
		if strings.TrimSpace(branchHead) != migration.Head {
			return fmt.Errorf("legacy local branch no longer matches the recorded migration HEAD")
		}
		if code, _, stderr = preflight.GitCmd(record.Repo, "branch", "-D", "--", record.Branch); code != 0 {
			return fmt.Errorf("remove verified legacy local branch: %s", strings.TrimSpace(stderr))
		}
	} else if code != 1 {
		return fmt.Errorf("inspect legacy local branch before migration: %s", strings.TrimSpace(stderr))
	}
	return nil
}

func markLegacyWorktreeGitRemoved(stateRoot, id string, expected IssueOpsLegacyWorktreeMigration, now string) (*IssueOpsLegacyWorktreeMigration, error) {
	var persisted IssueOpsLegacyWorktreeMigration
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if current.LegacyWorktreeMigration == nil || *current.LegacyWorktreeMigration != expected {
			return fmt.Errorf("legacy worktree migration changed during Git removal")
		}
		if current.LegacyWorktreeMigration.State == IssueOpsLegacyWorktreeMigrationStateGitRemoved {
			persisted = *current.LegacyWorktreeMigration
			return nil
		}
		if current.LegacyWorktreeMigration.State != IssueOpsLegacyWorktreeMigrationStatePrepared {
			return fmt.Errorf("legacy worktree migration is not ready for Git removal")
		}
		current.LegacyWorktreeMigration.State = IssueOpsLegacyWorktreeMigrationStateGitRemoved
		current.LegacyWorktreeMigration.GitRemovedAt = now
		current.UpdatedAt = now
		written, err := writeIssueOps(stateRoot, current)
		if err != nil {
			return err
		}
		persisted = *written.LegacyWorktreeMigration
		return nil
	})
	return &persisted, err
}

func createLegacyOrcaWorktree(ctx context.Context, record IssueOpsRecord, worktreePath string, probe port.OrcaProbeResult, baseRef string, client IssueOpsOrcaWorktreeClient) (port.OrcaWorktree, error) {
	linkedIssue, err := issueNumber(record.IssueURL)
	if err != nil {
		return port.OrcaWorktree{}, err
	}
	provider, err := issueOpsHandoffProvider(record)
	if err != nil {
		return port.OrcaWorktree{}, err
	}
	created, err := client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
		Repo: record.Repo, Name: record.Branch, BaseBranch: baseRef, Provider: provider, Issue: linkedIssue, Comment: issueOpsLegacyWorktreeMigrationMarker(record.ID),
	})
	if err != nil {
		return port.OrcaWorktree{}, fmt.Errorf("create Orca worktree from legacy migration: %w", err)
	}
	return created, nil
}

func containsExactMigratedWorktree(record IssueOpsRecord, worktreePath, repoID, baseRef string, created port.OrcaWorktree, rows []port.OrcaWorktree) bool {
	for _, row := range rows {
		if row.ID != created.ID || row.InstanceID != created.InstanceID {
			continue
		}
		if validateCreatedHandoffWorktree(record, worktreePath, repoID, baseRef, row) == nil && row.Comment == issueOpsLegacyWorktreeMigrationMarker(record.ID) {
			return true
		}
	}
	return false
}

func completeLegacyWorktreeMigration(stateRoot, id string, expected IssueOpsLegacyWorktreeMigration, identity *IssueOpsOrcaIdentity, now string) (*IssueOpsLegacyWorktreeMigration, error) {
	var persisted IssueOpsLegacyWorktreeMigration
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if current.LegacyWorktreeMigration == nil || *current.LegacyWorktreeMigration != expected {
			return fmt.Errorf("legacy worktree migration changed before Orca result persistence")
		}
		current.LegacyWorktreeMigration.State = IssueOpsLegacyWorktreeMigrationStateOrcaManaged
		current.LegacyWorktreeMigration.Orca = identity
		current.LegacyWorktreeMigration.CompletedAt = now
		current.UpdatedAt = now
		written, err := writeIssueOps(stateRoot, current)
		if err != nil {
			return err
		}
		persisted = *written.LegacyWorktreeMigration
		return nil
	})
	return &persisted, err
}

func migrationOrcaIdentity(probe port.OrcaProbeResult, baseRef string, worktree port.OrcaWorktree) *IssueOpsOrcaIdentity {
	return &IssueOpsOrcaIdentity{RuntimeID: probe.RuntimeID, RepoID: probe.RepoID, BaseRef: baseRef, WorktreeID: worktree.ID, WorktreeInstanceID: worktree.InstanceID, WorktreePath: filepath.Clean(worktree.Path)}
}
