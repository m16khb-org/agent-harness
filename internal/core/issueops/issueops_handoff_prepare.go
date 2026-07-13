package issueops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

const (
	IssueOpsOrchestratorAuto          = "auto"
	IssueOpsOrchestratorOrca          = "orca"
	IssueOpsOrchestratorInline        = "inline"
	IssueOpsInlineReasonUserRequested = "user-requested"
	IssueOpsInlineReasonRecovery      = "recovery"

	IssueOpsGitLabNativeMetadataUnavailableWarning = "orca_gitlab_native_metadata_unavailable"
)

type IssueOpsHandoffPrepareRequest struct {
	ID           string `json:"id"`
	Orchestrator string `json:"orchestrator,omitempty"`
	InlineReason string `json:"inline_reason,omitempty"`
	Agent        string `json:"agent,omitempty"`
	Confirm      bool   `json:"confirm,omitempty"`
}

type IssueOpsHandoffPrepareResult struct {
	OK            bool                  `json:"ok"`
	ID            string                `json:"id"`
	Repo          string                `json:"repo"`
	Branch        string                `json:"branch"`
	BaseBranch    string                `json:"base_branch"`
	WorktreePath  string                `json:"worktree_path"`
	Exists        bool                  `json:"exists"`
	Command       []string              `json:"command"`
	NextStep      string                `json:"next_step"`
	Preview       bool                  `json:"preview,omitempty"`
	RequestedMode string                `json:"requested_mode,omitempty"`
	ResolvedMode  string                `json:"resolved_mode,omitempty"`
	InlineReason  string                `json:"inline_reason,omitempty"`
	State         string                `json:"state,omitempty"`
	Attempt       int                   `json:"attempt,omitempty"`
	ContextSHA256 string                `json:"context_sha256,omitempty"`
	FallbackCode  string                `json:"fallback_code,omitempty"`
	RecoveryCode  string                `json:"recovery_code,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
	Orca          *IssueOpsOrcaIdentity `json:"orca,omitempty"`
}

type IssueOpsHandoffPrepareClock struct {
	Now      func() time.Time
	NewEpoch func() (string, error)
}

type IssueOpsOrcaWorktreeClient interface {
	Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error)
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
	CreateWorktree(context.Context, port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error)
}

func PrepareIssueOpsHandoffWorktree(ctx context.Context, stateRoot string, req IssueOpsHandoffPrepareRequest, client IssueOpsOrcaWorktreeClient, clock IssueOpsHandoffPrepareClock) (IssueOpsHandoffPrepareResult, error) {
	requested, err := normalizeOrchestrator(req.Orchestrator)
	if err != nil {
		return IssueOpsHandoffPrepareResult{}, err
	}
	inlineReason, err := validateInlineAuthorization(requested, req.InlineReason)
	if err != nil {
		return IssueOpsHandoffPrepareResult{}, err
	}
	req.InlineReason = inlineReason
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsHandoffPrepareResult{}, err
	}
	result, err := issueOpsLegacyWorktreePrepareResult(record)
	if err != nil {
		return result, err
	}
	result.RequestedMode = requested
	if record.ExecutionHandoff != nil {
		return existingHandoffPrepareResult(stateRoot, record, result, issueOpsHandoffNow(clock))
	}
	if requested != IssueOpsOrchestratorInline {
		agent, normalizeErr := handoff.NormalizeAgent(req.Agent)
		if normalizeErr != nil {
			return result, normalizeErr
		}
		req.Agent = agent
	}
	result, probe, proceed, err := resolveHandoffPrepareMode(ctx, record, req, requested, client, result)
	if err != nil || !proceed {
		return result, err
	}

	providerTrackingRef, err := issueOpsOrcaProviderTrackingRef(probe.RepoRemoteName, record.Branch)
	if err != nil {
		return result, err
	}
	if err := validateHandoffProviderRef(record, providerTrackingRef); err != nil {
		return result, err
	}
	worktrees, err := client.ListWorktrees(ctx, record.Repo)
	if err != nil {
		if requested == IssueOpsOrchestratorAuto {
			return inlineHandoffPrepareFallback(result, handoffInventoryFallbackCode(err)), nil
		}
		return result, fmt.Errorf("list Orca worktrees before create: %w", err)
	}
	baseline, err := handoff.CanonicalBaselineIDs("worktree", worktreeIDs(worktrees))
	if err != nil {
		return result, fmt.Errorf("Orca worktree baseline is unsafe: %w", err)
	}
	if err := handoff.RequireBaselineDeltaHeadroom("worktree", baseline); err != nil {
		return result, fmt.Errorf("Orca worktree baseline is unsafe: %w", err)
	}
	collisionCode, err := preflightHandoffWorktreeCreate(record, result.WorktreePath, worktrees)
	if err != nil {
		return result, err
	}
	if collisionCode != "" {
		if requested == IssueOpsOrchestratorAuto {
			return inlineHandoffPrepareFallback(result, collisionCode), nil
		}
		return result, fmt.Errorf("Orca worktree create collision: %s", collisionCode)
	}
	epoch, err := issueOpsHandoffEpoch(clock)
	if err != nil {
		return result, err
	}
	now := issueOpsHandoffNow(clock)
	fence := handoff.Fence{Attempt: 1, OwnershipEpoch: epoch}
	begin, err := beginHandoffWorktreeCreate(stateRoot, record.ID, result.WorktreePath, req.Agent, probe.RuntimeID, probe.RepoID, providerTrackingRef, fence, baseline, now)
	if err != nil {
		return result, err
	}
	if !begin.Authorized {
		current, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			return result, readErr
		}
		return existingHandoffPrepareResult(stateRoot, current, result, now)
	}

	created, err := createHandoffWorktree(ctx, stateRoot, record, result.WorktreePath, probe.RepoID, providerTrackingRef, baseline, fence, begin, client, now)
	if err != nil {
		if requested == IssueOpsOrchestratorAuto && externalMutationNotInvoked(err) {
			return inlineHandoffPrepareFallback(result, "orca_worktree_create_not_invoked"), nil
		}
		return result, err
	}
	linkStatus := providerIssueLinkStatus(record, created)
	persisted, err := persistHandoffWorktreeCreate(stateRoot, record.ID, created, linkStatus, fence, begin, now)
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "worktree_persist_failed", err.Error(), now)
		return result, err
	}
	return projectHandoffPrepareResult(result, persisted), nil
}

func preflightHandoffWorktreeCreate(record IssueOpsRecord, expectedPath string, rows []port.OrcaWorktree) (string, error) {
	expectedPath = filepath.Clean(expectedPath)
	base := filepath.Dir(expectedPath)
	baseInfo, err := os.Lstat(base)
	if err != nil {
		if os.IsNotExist(err) {
			return "orca_worktree_base_missing", nil
		}
		return "", fmt.Errorf("canonical Orca worktree base must already exist as a real directory: %w", err)
	}
	if baseInfo.Mode()&os.ModeSymlink != 0 || !baseInfo.IsDir() {
		return "", fmt.Errorf("canonical Orca worktree base must be a real non-symlink directory")
	}
	for _, row := range rows {
		pathCollision := strings.TrimSpace(row.Path) != "" && filepath.Clean(row.Path) == expectedPath
		branch := strings.TrimPrefix(strings.TrimSpace(row.Branch), "refs/heads/")
		branchCollision := branch != "" && branch == strings.TrimSpace(record.Branch)
		nameCollision := strings.TrimSpace(row.Name) != "" && strings.TrimSpace(row.Name) == strings.TrimSpace(record.Branch)
		if pathCollision || branchCollision || nameCollision {
			return "", fmt.Errorf("Orca inventory already contains the expected worktree path, branch, or name; reconcile or clean the exact Orca identity before create")
		}
	}
	leafInfo, err := os.Lstat(expectedPath)
	switch {
	case err == nil:
		if leafInfo.Mode()&os.ModeSymlink != 0 || !leafInfo.IsDir() {
			return "", fmt.Errorf("expected Orca worktree path already exists but is not a real directory")
		}
		if !existingLegacyWorktreeMatches(record, expectedPath) {
			return "", fmt.Errorf("expected Orca worktree path is occupied by a stale or mismatched checkout")
		}
		return "orca_existing_legacy_worktree", nil
	case !os.IsNotExist(err):
		return "", fmt.Errorf("inspect expected Orca worktree path: %w", err)
	}
	ref := "refs/heads/" + strings.TrimSpace(record.Branch)
	code, _, stderr := preflight.GitCmd(record.Repo, "show-ref", "--verify", "--quiet", ref)
	switch code {
	case 0:
		return "orca_local_branch_collision", nil
	case 1:
		return "", nil
	default:
		return "", fmt.Errorf("verify local provider branch collision: %s", strings.TrimSpace(stderr))
	}
}

func existingLegacyWorktreeMatches(record IssueOpsRecord, expectedPath string) bool {
	root := filepath.Clean(preflight.GitOut(expectedPath, "rev-parse", "--show-toplevel"))
	branch := strings.TrimSpace(preflight.GitOut(expectedPath, "branch", "--show-current"))
	head := strings.TrimSpace(preflight.GitOut(expectedPath, "rev-parse", "HEAD"))
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	resolvedExpected, expectedErr := filepath.EvalSymlinks(expectedPath)
	return rootErr == nil && expectedErr == nil && filepath.Clean(resolvedRoot) == filepath.Clean(resolvedExpected) && branch == strings.TrimSpace(record.Branch) && record.BranchPrepare != nil && head == strings.TrimSpace(record.BranchPrepare.BaseSHA)
}

func handoffInventoryFallbackCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "orca_worktree_inventory_timeout"
	}
	var orcaErr *port.OrcaError
	if errors.As(err, &orcaErr) {
		if orcaErr.Timeout {
			return "orca_worktree_inventory_timeout"
		}
		if orcaErr.Code == "incomplete_list" {
			return "orca_worktree_inventory_incomplete"
		}
	}
	return "orca_worktree_inventory_failed"
}

func resolveHandoffPrepareMode(ctx context.Context, record IssueOpsRecord, req IssueOpsHandoffPrepareRequest, requested string, client IssueOpsOrcaWorktreeClient, result IssueOpsHandoffPrepareResult) (IssueOpsHandoffPrepareResult, port.OrcaProbeResult, bool, error) {
	if requested == IssueOpsOrchestratorInline {
		result.ResolvedMode = IssueOpsOrchestratorInline
		result.InlineReason = req.InlineReason
		result.Preview = !req.Confirm
		return result, port.OrcaProbeResult{}, false, nil
	}
	providerHint := ""
	if record.BranchPrepare != nil {
		providerHint = strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
	}
	if providerHint == "github" || providerHint == "gitlab" {
		if _, err := issueOpsHandoffProvider(record); err != nil {
			return result, port.OrcaProbeResult{}, false, err
		}
	}
	if client == nil {
		if requested == IssueOpsOrchestratorAuto {
			return inlineHandoffPrepareFallback(result, "orca_adapter_unavailable"), port.OrcaProbeResult{}, false, nil
		}
		return result, port.OrcaProbeResult{}, false, fmt.Errorf("orca probe failed: adapter unavailable")
	}
	probe, err := client.Probe(ctx, port.OrcaProbeRequest{Repo: record.Repo, Agent: req.Agent, Provider: providerHint})
	if err != nil || !probe.Available || !probe.Ready {
		code := strings.TrimSpace(probe.Code)
		if code == "" {
			code = "orca_probe_failed"
		}
		if requested == IssueOpsOrchestratorAuto {
			return inlineHandoffPrepareFallback(result, code), probe, false, nil
		}
		if err != nil {
			return result, probe, false, fmt.Errorf("orca probe failed: %w", err)
		}
		return result, probe, false, fmt.Errorf("orca probe failed: %s", code)
	}
	provider, providerErr := issueOpsHandoffProvider(record)
	if providerErr != nil {
		if requested == IssueOpsOrchestratorAuto {
			return inlineHandoffPrepareFallback(result, "orca_provider_unsupported"), probe, false, nil
		}
		return result, probe, false, providerErr
	}
	result.ResolvedMode = IssueOpsOrchestratorOrca
	result.Preview = !req.Confirm
	if provider == "gitlab" {
		result.Warnings = uniqueStrings(append(result.Warnings, IssueOpsGitLabNativeMetadataUnavailableWarning))
	}
	if !req.Confirm {
		return result, probe, false, nil
	}
	if err := validateHandoffPreparePrerequisites(record); err != nil {
		return result, probe, false, err
	}
	return result, probe, true, nil
}

func inlineHandoffPrepareFallback(result IssueOpsHandoffPrepareResult, code string) IssueOpsHandoffPrepareResult {
	_ = code
	result.Preview = false
	result.RequestedMode = ""
	result.ResolvedMode = ""
	result.State = ""
	result.Attempt = 0
	result.ContextSHA256 = ""
	result.FallbackCode = ""
	result.RecoveryCode = ""
	result.Warnings = nil
	result.Orca = nil
	return result
}

func removeString(values []string, remove string) []string {
	filtered := values[:0]
	for _, value := range values {
		if value != remove {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

type handoffWorktreeBeginSnapshot struct {
	Authorized        bool
	ExpectedJournal   IssueOpsRecord
	PreviousUpdatedAt string
}

func beginHandoffWorktreeCreate(stateRoot, id, workerRoot, agent, runtimeID, repoID, baseRef string, fence handoff.Fence, baseline []string, now string) (handoffWorktreeBeginSnapshot, error) {
	snapshot := handoffWorktreeBeginSnapshot{}
	err := withIssueOpsLock(stateRoot, id, func() error {
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if current.ExecutionHandoff != nil {
			return nil
		}
		snapshot.PreviousUpdatedAt = current.UpdatedAt
		current, err = handoff.Prepare(current, handoff.PrepareRequest{
			Attempt: fence.Attempt, OwnershipEpoch: fence.OwnershipEpoch, CoordinatorRoot: current.Repo,
			AttemptBaseHead: current.BranchPrepare.BaseSHA, WorkerRoot: workerRoot, Agent: agent, Now: now,
		})
		if err != nil {
			return err
		}
		current.ExecutionHandoff.Orca = &model.IssueOpsOrcaIdentity{RuntimeID: strings.TrimSpace(runtimeID), RepoID: strings.TrimSpace(repoID), BaseRef: strings.TrimSpace(baseRef)}
		current, err = handoff.BeginOperation(current, fence, model.IssueOpsExecutionHandoffPendingOperation{
			Kind: handoff.OperationWorktreeCreate, StartedAt: now, BaselineWorktreeIDs: baseline,
		})
		if err != nil {
			return err
		}
		current.UpdatedAt = now
		persisted, err := writeIssueOps(stateRoot, current)
		if err == nil {
			snapshot.Authorized = true
			snapshot.ExpectedJournal = persisted
		}
		return err
	})
	return snapshot, err
}

func createHandoffWorktree(ctx context.Context, stateRoot string, record IssueOpsRecord, expectedPath, expectedRepoID, providerTrackingRef string, baseline []string, fence handoff.Fence, begin handoffWorktreeBeginSnapshot, client IssueOpsOrcaWorktreeClient, now string) (port.OrcaWorktree, error) {
	linkedIssue, _ := issueNumber(record.IssueURL)
	provider, _ := issueOpsHandoffProvider(record)
	created, err := client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
		Repo: record.Repo, Name: record.Branch, BaseBranch: providerTrackingRef, Provider: provider, Issue: linkedIssue,
		Comment: issueOpsHandoffMarker(record.ID, fence.OwnershipEpoch, fence.Attempt),
	})
	if err != nil {
		if externalMutationNotInvoked(err) {
			if rollbackErr := rollbackHandoffWorktreeStartFailure(stateRoot, record.ID, begin); rollbackErr != nil {
				return port.OrcaWorktree{}, fmt.Errorf("clear non-invoked Orca worktree create journal: %w", rollbackErr)
			}
			return port.OrcaWorktree{}, fmt.Errorf("Orca worktree create was not invoked and is safe to retry: %w", err)
		}
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "worktree_create_ambiguous", err.Error(), now)
		return port.OrcaWorktree{}, fmt.Errorf("Orca worktree create requires recovery: %w", err)
	}
	validationErr := validateCreatedHandoffWorktree(record, expectedPath, expectedRepoID, providerTrackingRef, created)
	if validationErr == nil && created.Comment != issueOpsHandoffMarker(record.ID, fence.OwnershipEpoch, fence.Attempt) {
		validationErr = fmt.Errorf("Orca worktree response does not contain the exact attempt marker")
	}
	if validationErr != nil {
		if worktreeCleanupCandidateExact(record, expectedRepoID, fence, baseline, created) {
			_ = markHandoffCleanupOnlyWorktree(stateRoot, record.ID, fence, created, validationErr.Error(), now)
		} else {
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "worktree_identity_mismatch", validationErr.Error(), now)
		}
		return port.OrcaWorktree{}, validationErr
	}
	return created, nil
}

func worktreeCleanupCandidateExact(record IssueOpsRecord, expectedRepoID string, fence handoff.Fence, baseline []string, created port.OrcaWorktree) bool {
	if strings.TrimSpace(created.ID) == "" || strings.TrimSpace(created.InstanceID) == "" || strings.TrimSpace(created.RepoID) == "" || created.RepoID != strings.TrimSpace(expectedRepoID) {
		return false
	}
	if created.Comment != issueOpsHandoffMarker(record.ID, fence.OwnershipEpoch, fence.Attempt) {
		return false
	}
	for _, existing := range baseline {
		if strings.TrimSpace(existing) == created.ID {
			return false
		}
	}
	return true
}

func rollbackHandoffWorktreeStartFailure(stateRoot, id string, begin handoffWorktreeBeginSnapshot) error {
	return withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if !begin.Authorized || !reflect.DeepEqual(record, begin.ExpectedJournal) {
			return fmt.Errorf("worktree create journal changed before definitive rollback")
		}
		record.ExecutionHandoff = nil
		record.UpdatedAt = begin.PreviousUpdatedAt
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

func externalMutationNotInvoked(err error) bool {
	var orcaErr *port.OrcaError
	return errors.As(err, &orcaErr) && !orcaErr.Invoked
}

func markHandoffCleanupOnlyWorktree(stateRoot, id string, fence handoff.Fence, created port.OrcaWorktree, reason, now string) error {
	return withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		record, err = handoff.MarkCleanupOnlyWorktree(record, fence, model.IssueOpsOrcaCleanupArtifact{
			Kind: "worktree", ID: created.ID, InstanceID: created.InstanceID, Path: created.Path, Reason: reason,
		}, model.IssueOpsExecutionHandoffFailure{Code: "worktree_cleanup_only", Message: reason, At: now})
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

func persistHandoffWorktreeCreate(stateRoot, id string, created port.OrcaWorktree, linkStatus string, fence handoff.Fence, begin handoffWorktreeBeginSnapshot, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if current.ExecutionHandoff == nil || current.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || current.ExecutionHandoff.PendingOperation == nil || current.ExecutionHandoff.PendingOperation.Kind != handoff.OperationWorktreeCreate {
			return fmt.Errorf("worktree create result lost its pending-operation fence")
		}
		if !begin.Authorized || !reflect.DeepEqual(current, begin.ExpectedJournal) {
			return fmt.Errorf("worktree create authorized journal changed before result persist")
		}
		if current.ExecutionHandoff.Attempt != fence.Attempt || current.ExecutionHandoff.OwnershipEpoch != fence.OwnershipEpoch {
			return fmt.Errorf("stale worktree create result")
		}
		current.ExecutionHandoff.Orca = &model.IssueOpsOrcaIdentity{
			RuntimeID: current.ExecutionHandoff.Orca.RuntimeID, RepoID: current.ExecutionHandoff.Orca.RepoID, BaseRef: current.ExecutionHandoff.Orca.BaseRef, ProviderIssueLinkStatus: linkStatus,
			WorktreeID: created.ID, WorktreeInstanceID: created.InstanceID, WorktreePath: filepath.Clean(created.Path),
		}
		current.ExecutionHandoff.ProvisionedAt = now
		current.WorktreePath = filepath.Clean(created.Path)
		current, err = handoff.CompleteOperation(current, fence, handoff.OperationWorktreeCreate, now)
		if err != nil {
			return err
		}
		current.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, current)
		return err
	})
	return persisted, err
}

func issueOpsOrcaProviderTrackingRef(remoteName, branch string) (string, error) {
	remoteName = strings.TrimSpace(remoteName)
	branch = strings.TrimSpace(branch)
	if remoteName == "" || branch == "" {
		return "", fmt.Errorf("Orca repo remote name and verified provider branch are required")
	}
	return "refs/remotes/" + remoteName + "/" + branch, nil
}

func issueOpsLegacyWorktreePrepareResult(record IssueOpsRecord) (IssueOpsHandoffPrepareResult, error) {
	repo := strings.TrimSpace(record.Repo)
	branch := strings.TrimSpace(record.Branch)
	if repo == "" || branch == "" {
		return IssueOpsHandoffPrepareResult{}, fmt.Errorf("repo and branch must be set on the IssueOps record")
	}
	baseBranch := "main"
	if record.BranchPrepare != nil {
		if strings.TrimSpace(record.BranchPrepare.BaseBranch) != "" {
			baseBranch = strings.TrimSpace(record.BranchPrepare.BaseBranch)
		}
	}
	path := repo + ".worktrees/" + strings.ReplaceAll(branch, "/", "-")
	result := IssueOpsHandoffPrepareResult{
		OK: true, ID: record.ID, Repo: repo, Branch: branch, BaseBranch: baseBranch, WorktreePath: path,
		Command:  []string{"git", "worktree", "add", path, branch},
		NextStep: "execute the command above, then run issueops link-worktree --id " + record.ID + " --worktree-path " + path,
	}
	if info, err := os.Stat(result.WorktreePath); err == nil && info.IsDir() {
		result.Exists = true
		result.NextStep = "worktree exists; run issueops link-worktree --id " + record.ID + " --worktree-path " + result.WorktreePath
	}
	return result, nil
}

func providerIssueLinkStatus(record IssueOpsRecord, created port.OrcaWorktree) string {
	provider, err := issueOpsHandoffProvider(record)
	if err != nil || provider != "gitlab" {
		return ""
	}
	issue, err := issueNumber(record.IssueURL)
	if err == nil && created.GitLabIssue != nil && *created.GitLabIssue == issue {
		return handoff.ProviderIssueLinkGitLabExact
	}
	return handoff.ProviderIssueLinkGitLabUnavailable
}

func validateHandoffPreparePrerequisites(record IssueOpsRecord) error {
	missing := make([]string, 0, 4)
	if record.DesignReview == nil || !record.DesignReview.Approved {
		missing = append(missing, "approved_design_review")
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "linked_issue")
	}
	if record.BranchPrepare == nil || !record.BranchPrepare.LinkVerified || record.BranchPrepare.Branch != record.Branch || record.BranchPrepare.IssueURL != record.IssueURL {
		missing = append(missing, "verified_provider_branch")
	} else if !validFullCommitSHA(record.BranchPrepare.BaseSHA) {
		missing = append(missing, "resolved_provider_base_sha")
	}
	if len(missing) > 0 {
		return fmt.Errorf("Orca worktree preparation prerequisites missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func issueOpsHandoffProvider(record IssueOpsRecord) (string, error) {
	provider := ""
	branchIssueURL := ""
	if record.BranchPrepare != nil {
		provider = strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
		branchIssueURL = strings.TrimSpace(record.BranchPrepare.IssueURL)
	}
	if provider != "github" && provider != "gitlab" {
		return "", fmt.Errorf("Orca supervised handoff requires provider github or gitlab; got %q", provider)
	}
	issueURL := strings.TrimSpace(record.IssueURL)
	if issueProvider := remote.ProviderFromURL(issueURL); issueProvider == "" || issueProvider != provider || branchIssueURL != issueURL {
		return "", fmt.Errorf("verified provider does not match IssueOps issue URL")
	}
	return provider, nil
}

func validFullCommitSHA(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateHandoffProviderRef(record IssueOpsRecord, providerTrackingRef string) error {
	if record.BranchPrepare == nil || !validFullCommitSHA(record.BranchPrepare.BaseSHA) {
		return fmt.Errorf("Orca preparation requires a nonempty resolved provider base commit SHA")
	}
	want := strings.ToLower(strings.TrimSpace(record.BranchPrepare.BaseSHA))
	code, base, _ := preflight.GitCmd(record.Repo, "rev-parse", "--verify", want+"^{commit}")
	if code != 0 || strings.ToLower(strings.TrimSpace(base)) != want {
		return fmt.Errorf("prepared base SHA is missing or stale in the local repository")
	}
	code, tracked, _ := preflight.GitCmd(record.Repo, "rev-parse", "--verify", strings.TrimSpace(providerTrackingRef)+"^{commit}")
	if code != 0 || strings.ToLower(strings.TrimSpace(tracked)) != want {
		return fmt.Errorf("verified provider tracking ref does not resolve to the prepared base SHA")
	}
	return nil
}

func validateCreatedHandoffWorktree(record IssueOpsRecord, expectedPath, expectedRepoID, expectedBaseRef string, created port.OrcaWorktree) error {
	if strings.TrimSpace(created.ID) == "" || strings.TrimSpace(created.InstanceID) == "" {
		return fmt.Errorf("Orca worktree id and instance id are required")
	}
	if strings.TrimSpace(expectedRepoID) == "" || strings.TrimSpace(created.RepoID) == "" || created.RepoID != expectedRepoID {
		return fmt.Errorf("Orca worktree repo identity does not match the probed repository")
	}
	if strings.TrimSpace(expectedBaseRef) == "" || strings.TrimSpace(created.BaseRef) == "" || created.BaseRef != expectedBaseRef {
		return fmt.Errorf("Orca worktree base ref does not match the verified provider tracking ref")
	}
	expectedClean := filepath.Clean(strings.TrimSpace(expectedPath))
	createdClean := filepath.Clean(strings.TrimSpace(created.Path))
	if expectedClean == "." || createdClean != expectedClean {
		return fmt.Errorf("Orca worktree path %q is not the exact canonical IssueOps path %q; set Orca Settings > General > Workspace > Nest Workspaces to OFF, verify the provider tracking ref, then cancel, remove the exact mismatched resource, and start a fresh IssueOps cycle", created.Path, expectedPath)
	}
	for _, path := range []string{filepath.Dir(expectedClean), expectedClean} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("canonical IssueOps worktree path must be a real directory without a symlink leaf or worktree-base component")
		}
	}
	resolvedExpected, expectedErr := filepath.EvalSymlinks(expectedPath)
	resolvedCreated, createdErr := filepath.EvalSymlinks(created.Path)
	if expectedErr != nil || createdErr != nil || filepath.Clean(resolvedCreated) != filepath.Clean(resolvedExpected) {
		return fmt.Errorf("Orca worktree path %q does not match canonical IssueOps path %q; set Orca Settings > General > Workspace > Nest Workspaces to OFF and verify the provider tracking ref is selected as the Orca base branch, then cancel and remove the mismatched handoff resources and start a fresh IssueOps cycle", created.Path, expectedPath)
	}
	if strings.TrimPrefix(strings.TrimSpace(created.Branch), "refs/heads/") != strings.TrimSpace(record.Branch) {
		return fmt.Errorf("Orca worktree branch %q does not match %q", created.Branch, record.Branch)
	}
	if base := strings.TrimSpace(record.BranchPrepare.BaseSHA); base != "" && strings.TrimSpace(created.Head) != base {
		return fmt.Errorf("Orca worktree head does not match prepared base sha")
	}
	if err := validateHandoffWorktreeIssueMetadata(record, created); err != nil {
		return err
	}
	code, localRoot, _ := preflight.GitCmd(created.Path, "rev-parse", "--show-toplevel")
	if code != 0 || filepath.Clean(localRoot) != filepath.Clean(resolvedExpected) {
		return fmt.Errorf("Orca worktree path is not the expected local Git checkout")
	}
	code, localHead, _ := preflight.GitCmd(created.Path, "rev-parse", "--verify", "HEAD^{commit}")
	if code != 0 || strings.TrimSpace(localHead) != strings.TrimSpace(record.BranchPrepare.BaseSHA) || strings.TrimSpace(localHead) != strings.TrimSpace(created.Head) {
		return fmt.Errorf("Orca worktree local HEAD does not match the prepared base sha and response")
	}
	if branch := strings.TrimSpace(preflight.GitOut(created.Path, "branch", "--show-current")); branch == "" || branch != strings.TrimSpace(record.Branch) {
		return fmt.Errorf("Orca worktree local branch does not match the provider branch")
	}
	return nil
}

func validateHandoffWorktreeIssueMetadata(record IssueOpsRecord, created port.OrcaWorktree) error {
	issue, err := issueNumber(record.IssueURL)
	if err != nil {
		return fmt.Errorf("IssueOps issue URL does not contain a numeric issue identity")
	}
	provider, err := issueOpsHandoffProvider(record)
	if err != nil {
		return err
	}
	switch provider {
	case "github":
		if created.Issue != issue {
			return fmt.Errorf("Orca linked issue %d does not match IssueOps issue", created.Issue)
		}
		if created.GitLabIssue != nil && *created.GitLabIssue != 0 {
			return fmt.Errorf("Orca linked GitLab issue metadata conflicts with the GitHub provider")
		}
	case "gitlab":
		if created.Issue != 0 {
			return fmt.Errorf("Orca GitHub linked issue metadata must be absent for GitLab handoff")
		}
		if created.GitLabIssue != nil && (*created.GitLabIssue < 0 || *created.GitLabIssue > 0 && *created.GitLabIssue != issue) {
			return fmt.Errorf("Orca linked GitLab issue does not match IssueOps issue")
		}
	}
	return nil
}

func existingHandoffPrepareResult(stateRoot string, record IssueOpsRecord, result IssueOpsHandoffPrepareResult, now string) (IssueOpsHandoffPrepareResult, error) {
	if record.ExecutionHandoff != nil && record.ExecutionHandoff.PendingOperation != nil && record.ExecutionHandoff.State != handoff.StateRecoveryRequired {
		fence := handoff.Fence{Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256}
		if err := markHandoffPrepareRecovery(stateRoot, record.ID, fence, "pending_operation_requires_recovery", "restart observed an unresolved external mutation", now); err != nil {
			return result, err
		}
		var err error
		record, err = ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return result, err
		}
	}
	return projectHandoffPrepareResult(result, record), nil
}

func projectHandoffPrepareResult(result IssueOpsHandoffPrepareResult, record IssueOpsRecord) IssueOpsHandoffPrepareResult {
	result.Preview = false
	result.ResolvedMode = IssueOpsOrchestratorOrca
	result.Command = nil
	result.NextStep = "inspect the persisted Orca handoff with issueops resume --repo " + record.Repo + " --id " + record.ID
	if record.ExecutionHandoff == nil {
		return result
	}
	result.State = record.ExecutionHandoff.State
	result.Attempt = record.ExecutionHandoff.Attempt
	result.ContextSHA256 = record.ExecutionHandoff.ContextSHA256
	result.Orca = record.ExecutionHandoff.Orca
	if provider, err := issueOpsHandoffProvider(record); err == nil && provider == "gitlab" && record.ExecutionHandoff.Orca != nil {
		if record.ExecutionHandoff.Orca.ProviderIssueLinkStatus == handoff.ProviderIssueLinkGitLabExact {
			result.Warnings = removeString(result.Warnings, IssueOpsGitLabNativeMetadataUnavailableWarning)
		} else {
			result.Warnings = uniqueStrings(append(result.Warnings, IssueOpsGitLabNativeMetadataUnavailableWarning))
		}
	}
	if record.ExecutionHandoff.State == handoff.StateRecoveryRequired {
		result.RecoveryCode = "explicit_reconcile_required"
		if record.ExecutionHandoff.Failure != nil && record.ExecutionHandoff.Failure.Code != "" {
			result.RecoveryCode = record.ExecutionHandoff.Failure.Code
		}
	}
	return result
}

func markHandoffPrepareRecovery(stateRoot, id string, fence handoff.Fence, code, message, now string) error {
	return withIssueOpsLock(stateRoot, id, func() error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff == nil || record.ExecutionHandoff.State == handoff.StateRecoveryRequired {
			return nil
		}
		record, err = handoff.MarkRecoveryRequired(record, fence, model.IssueOpsExecutionHandoffFailure{Code: code, Message: message, At: now})
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

func ReconcileIssueOpsHandoffWorktree(pending IssueOpsExecutionHandoffPendingOperation, id, epoch string, attempt int, rows []port.OrcaWorktree) (port.OrcaWorktree, error) {
	if pending.Kind != handoff.OperationWorktreeCreate {
		return port.OrcaWorktree{}, fmt.Errorf("pending operation is not worktree_create")
	}
	baseline := make(map[string]struct{}, len(pending.BaselineWorktreeIDs))
	for _, id := range pending.BaselineWorktreeIDs {
		baseline[id] = struct{}{}
	}
	marker := issueOpsHandoffMarker(id, epoch, attempt)
	candidates := make([]port.OrcaWorktree, 0, 1)
	for _, row := range rows {
		if _, existed := baseline[row.ID]; existed || row.Comment != marker {
			continue
		}
		candidates = append(candidates, row)
	}
	if len(candidates) != 1 {
		return port.OrcaWorktree{}, fmt.Errorf("worktree recovery requires exactly one marker candidate; found %d", len(candidates))
	}
	return candidates[0], nil
}

func normalizeOrchestrator(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = IssueOpsOrchestratorAuto
	}
	switch value {
	case IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca, IssueOpsOrchestratorInline:
		return value, nil
	default:
		return "", fmt.Errorf("orchestrator must be auto, orca, or inline")
	}
}

func validateInlineAuthorization(orchestrator, reason string) (string, error) {
	if orchestrator != IssueOpsOrchestratorInline {
		if reason != "" {
			return "", fmt.Errorf("--inline-reason is valid only with --orchestrator inline")
		}
		return "", nil
	}
	if reason == "" {
		return "", fmt.Errorf("explicit inline requires --inline-reason user-requested|recovery")
	}
	switch reason {
	case IssueOpsInlineReasonUserRequested, IssueOpsInlineReasonRecovery:
		return reason, nil
	default:
		return "", fmt.Errorf("inline reason must be user-requested or recovery")
	}
}

func issueOpsHandoffNow(clock IssueOpsHandoffPrepareClock) string {
	if clock.Now != nil {
		return clock.Now().UTC().Format(time.RFC3339Nano)
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func issueOpsHandoffEpoch(clock IssueOpsHandoffPrepareClock) (string, error) {
	if clock.NewEpoch != nil {
		return clock.NewEpoch()
	}
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create ownership epoch: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func issueOpsHandoffMarker(id, epoch string, attempt int) string {
	return "agent-harness issueops=" + strings.TrimSpace(id) + " ownership=" + strings.TrimSpace(epoch) + " attempt=" + strconv.Itoa(attempt)
}

func worktreeIDs(rows []port.OrcaWorktree) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := strings.TrimSpace(row.ID); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func issueNumber(raw string) (int, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("issue url is missing a number")
	}
	return strconv.Atoi(parts[len(parts)-1])
}
