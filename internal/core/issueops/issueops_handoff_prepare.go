package issueops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

const (
	IssueOpsOrchestratorAuto   = "auto"
	IssueOpsOrchestratorOrca   = "orca"
	IssueOpsOrchestratorInline = "inline"
)

type IssueOpsHandoffPrepareRequest struct {
	ID           string `json:"id"`
	Orchestrator string `json:"orchestrator,omitempty"`
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
	State         string                `json:"state,omitempty"`
	Attempt       int                   `json:"attempt,omitempty"`
	ContextSHA256 string                `json:"context_sha256,omitempty"`
	FallbackCode  string                `json:"fallback_code,omitempty"`
	RecoveryCode  string                `json:"recovery_code,omitempty"`
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
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsHandoffPrepareResult{}, err
	}
	result, err := issueOpsLegacyWorktreePrepareResult(record)
	if err != nil {
		return result, err
	}
	requested, err := normalizeOrchestrator(req.Orchestrator)
	if err != nil {
		return result, err
	}
	result.RequestedMode = requested
	if requested == IssueOpsOrchestratorInline {
		result.ResolvedMode = IssueOpsOrchestratorInline
		result.Preview = !req.Confirm
		return result, nil
	}

	if client == nil {
		if requested == IssueOpsOrchestratorAuto {
			result.ResolvedMode = IssueOpsOrchestratorInline
			result.FallbackCode = "orca_adapter_unavailable"
			return result, nil
		}
		return result, fmt.Errorf("orca probe failed: adapter unavailable")
	}
	probe, probeErr := client.Probe(ctx, port.OrcaProbeRequest{Repo: record.Repo, Agent: req.Agent})
	if probeErr != nil || !probe.Available || !probe.Ready {
		code := strings.TrimSpace(probe.Code)
		if code == "" {
			code = "orca_probe_failed"
		}
		if requested == IssueOpsOrchestratorAuto {
			result.ResolvedMode = IssueOpsOrchestratorInline
			result.FallbackCode = code
			return result, nil
		}
		if probeErr != nil {
			return result, fmt.Errorf("orca probe failed: %w", probeErr)
		}
		return result, fmt.Errorf("orca probe failed: %s", code)
	}
	result.ResolvedMode = IssueOpsOrchestratorOrca
	result.Preview = !req.Confirm
	if !req.Confirm {
		return result, nil
	}
	if err := validateHandoffPreparePrerequisites(record); err != nil {
		return result, err
	}

	if record.ExecutionHandoff != nil {
		return existingHandoffPrepareResult(stateRoot, record, result, issueOpsHandoffNow(clock))
	}
	providerTrackingRef, err := issueOpsOrcaProviderTrackingRef(probe.RepoRemoteName, record.Branch)
	if err != nil {
		return result, err
	}
	worktrees, err := client.ListWorktrees(ctx, record.Repo)
	if err != nil {
		return result, fmt.Errorf("list Orca worktrees before create: %w", err)
	}
	baseline := worktreeIDs(worktrees)
	epoch, err := issueOpsHandoffEpoch(clock)
	if err != nil {
		return result, err
	}
	now := issueOpsHandoffNow(clock)
	fence := handoff.Fence{Attempt: 1, OwnershipEpoch: epoch}
	prepared := false
	err = withIssueOpsLock(stateRoot, record.ID, func() error {
		current, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			return readErr
		}
		if current.ExecutionHandoff != nil {
			return nil
		}
		current, readErr = handoff.Prepare(current, handoff.PrepareRequest{
			Attempt: 1, OwnershipEpoch: epoch, CoordinatorRoot: current.Repo,
			WorkerRoot: result.WorktreePath, Agent: req.Agent, Now: now,
		})
		if readErr != nil {
			return readErr
		}
		current, readErr = handoff.BeginOperation(current, fence, model.IssueOpsExecutionHandoffPendingOperation{
			Kind: handoff.OperationWorktreeCreate, StartedAt: now, BaselineWorktreeIDs: baseline,
		})
		if readErr != nil {
			return readErr
		}
		current.UpdatedAt = now
		_, readErr = writeIssueOps(stateRoot, current)
		prepared = readErr == nil
		return readErr
	})
	if err != nil {
		return result, err
	}
	if !prepared {
		current, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			return result, readErr
		}
		return existingHandoffPrepareResult(stateRoot, current, result, now)
	}

	marker := issueOpsHandoffMarker(record.ID, epoch, 1)
	linkedIssue, _ := issueNumber(record.IssueURL)
	created, createErr := client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
		Repo: record.Repo, Name: record.Branch, BaseBranch: providerTrackingRef, Issue: linkedIssue, Comment: marker,
	})
	if createErr != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "worktree_create_ambiguous", createErr.Error(), now)
		return result, fmt.Errorf("Orca worktree create requires recovery: %w", createErr)
	}
	if err := validateCreatedHandoffWorktree(record, result.WorktreePath, created); err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "worktree_identity_mismatch", err.Error(), now)
		return result, err
	}

	var persisted IssueOpsRecord
	err = withIssueOpsLock(stateRoot, record.ID, func() error {
		current, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			return readErr
		}
		if current.ExecutionHandoff == nil || current.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || current.ExecutionHandoff.PendingOperation == nil || current.ExecutionHandoff.PendingOperation.Kind != handoff.OperationWorktreeCreate {
			return fmt.Errorf("worktree create result lost its pending-operation fence")
		}
		if current.ExecutionHandoff.Attempt != fence.Attempt || current.ExecutionHandoff.OwnershipEpoch != fence.OwnershipEpoch {
			return fmt.Errorf("stale worktree create result")
		}
		current.ExecutionHandoff.Orca = &model.IssueOpsOrcaIdentity{
			RuntimeID: probe.RuntimeID, WorktreeID: created.ID, WorktreeInstanceID: created.InstanceID, WorktreePath: filepath.Clean(created.Path),
		}
		current.ExecutionHandoff.ProvisionedAt = now
		current.WorktreePath = filepath.Clean(created.Path)
		current, readErr = handoff.CompleteOperation(current, fence, handoff.OperationWorktreeCreate, now)
		if readErr != nil {
			return readErr
		}
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "worktree_persist_failed", err.Error(), now)
		return result, err
	}
	return projectHandoffPrepareResult(result, persisted), nil
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
	if record.BranchPrepare != nil && strings.TrimSpace(record.BranchPrepare.BaseBranch) != "" {
		baseBranch = strings.TrimSpace(record.BranchPrepare.BaseBranch)
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
	}
	if len(missing) > 0 {
		return fmt.Errorf("Orca worktree preparation prerequisites missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateCreatedHandoffWorktree(record IssueOpsRecord, expectedPath string, created port.OrcaWorktree) error {
	if strings.TrimSpace(created.ID) == "" || strings.TrimSpace(created.InstanceID) == "" {
		return fmt.Errorf("Orca worktree id and instance id are required")
	}
	if filepath.Clean(created.Path) != filepath.Clean(expectedPath) {
		return fmt.Errorf("Orca worktree path %q does not match canonical IssueOps path %q; set Orca Settings > General > Workspace > Nest Workspaces to OFF and verify the provider tracking ref is selected as the Orca base branch, then cancel and remove the mismatched handoff resources and start a fresh IssueOps cycle", created.Path, expectedPath)
	}
	if strings.TrimPrefix(strings.TrimSpace(created.Branch), "refs/heads/") != strings.TrimSpace(record.Branch) {
		return fmt.Errorf("Orca worktree branch %q does not match %q", created.Branch, record.Branch)
	}
	if base := strings.TrimSpace(record.BranchPrepare.BaseSHA); base != "" && strings.TrimSpace(created.Head) != base {
		return fmt.Errorf("Orca worktree head does not match prepared base sha")
	}
	issue, err := issueNumber(record.IssueURL)
	if err != nil || created.Issue != issue {
		return fmt.Errorf("Orca linked issue %d does not match IssueOps issue", created.Issue)
	}
	if !issueOpsWorktreePathValid(created.Path) {
		return fmt.Errorf("Orca worktree path does not exist: %s", created.Path)
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
	if record.ExecutionHandoff == nil {
		return result
	}
	result.State = record.ExecutionHandoff.State
	result.Attempt = record.ExecutionHandoff.Attempt
	result.ContextSHA256 = record.ExecutionHandoff.ContextSHA256
	result.Orca = record.ExecutionHandoff.Orca
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

func ReconcileIssueOpsHandoffWorktree(pending IssueOpsExecutionHandoffPendingOperation, epoch string, attempt int, rows []port.OrcaWorktree) (port.OrcaWorktree, error) {
	if pending.Kind != handoff.OperationWorktreeCreate {
		return port.OrcaWorktree{}, fmt.Errorf("pending operation is not worktree_create")
	}
	baseline := make(map[string]struct{}, len(pending.BaselineWorktreeIDs))
	for _, id := range pending.BaselineWorktreeIDs {
		baseline[id] = struct{}{}
	}
	marker := "ownership=" + strings.TrimSpace(epoch) + " attempt=" + strconv.Itoa(attempt)
	candidates := make([]port.OrcaWorktree, 0, 1)
	for _, row := range rows {
		if _, existed := baseline[row.ID]; existed || !strings.Contains(row.Comment, marker) {
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
