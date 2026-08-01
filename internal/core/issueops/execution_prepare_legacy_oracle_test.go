package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type ExecutionPrepareDependencies struct {
	Direct      port.ExecutionWorkspaceProvisioner
	Orca        port.ExecutionOrcaProvisioner
	ReadIssue   ExecutionIssueSnapshotReadFunc
	Now         func() time.Time
	OperationID string
}

// PrepareExecution remains test-only so the predecessor characterization suite
// cannot become a production routing path again.
func PrepareExecution(ctx context.Context, stateRoot string, request ExecutionPrepareRequest, dependencies ExecutionPrepareDependencies) (ExecutionPrepareResult, error) {
	return prepareExecutionCompatibilityOracle(ctx, stateRoot, request, dependencies)
}

// These three functions freeze the predecessor orchestration for deterministic
// differential tests. They intentionally call only granular compatibility
// effects that remain available during the vertical cutover.
func prepareExecutionCompatibilityOracle(ctx context.Context, stateRoot string, req ExecutionPrepareRequest, deps ExecutionPrepareDependencies) (ExecutionPrepareResult, error) {
	if defaultModel, defaultEffort, ok := port.IssueOpsImplementerDefaults(strings.ToLower(strings.TrimSpace(req.OwnerHost))); ok {
		if strings.TrimSpace(req.OwnerModel) == "" {
			req.OwnerModel = defaultModel
		}
		if strings.TrimSpace(req.OwnerEffort) == "" {
			req.OwnerEffort = defaultEffort
		}
	}
	if req.Confirm {
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionPrepareResult{OK: false, ID: req.ID}, err
		}
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID}, err
	}
	requested, err := normalizeExecutionPrepareMode(req.Mode)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID}, err
	}
	if record.Execution != nil {
		if record.Execution.Pending != nil {
			result := preparedExecutionResult(record, requested)
			result.OK = false
			result.NextCommand = executionReconcileCommand(record.ID, true)
			return result, fmt.Errorf("IssueOps execution has a pending external intent; run %s", result.NextCommand)
		}
		if requested != ExecutionModeAuto && requested != string(record.Execution.Mode) {
			result := preparedExecutionResult(record, requested)
			result.OK = false
			result.NextCommand = executionSwitchModeCommand(record.ID, requested)
			return result, fmt.Errorf(
				"IssueOps execution is already prepared as %s; switching to %s removes the canonical worktree, so run %s",
				record.Execution.Mode, requested, result.NextCommand)
		}
		if next, err := executionWriterAbsentNextCommand(record, req.Confirm); err != nil {
			result := preparedExecutionResult(record, requested)
			result.OK = false
			result.NextCommand = next
			return result, err
		}
		return preparedExecutionResult(record, requested), nil
	}
	workspaceReq, err := executionWorkspaceRequest(record, req.Confirm)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID, RequestedMode: requested}, err
	}
	if err := ensureExecutionRootUnclaimed(stateRoot, record.ID, workspaceReq.Root); err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID, RequestedMode: requested}, err
	}
	resolved, fallback, probe, err := resolveExecutionPrepareMode(ctx, record, req, requested, deps.Orca)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID, RequestedMode: requested}, err
	}
	result := ExecutionPrepareResult{
		OK: true, ID: record.ID, Preview: !req.Confirm,
		RequestedMode: requested, ResolvedMode: resolved, FallbackCode: fallback,
		Workspace: model.Workspace{
			SourceRoot: workspaceReq.SourceRoot, Root: workspaceReq.Root, Branch: workspaceReq.Branch,
			BaseHead: workspaceReq.BaseHead, ParentWorktree: workspaceReq.ParentWorktree,
			Driver: map[string]string{"direct": "git", "orca": "orca"}[resolved],
		},
	}
	if resolved == string(model.ExecutionModeDirect) {
		return prepareDirectExecutionCompatibilityOracle(ctx, stateRoot, record, req, deps, workspaceReq, result)
	}
	return prepareOrcaExecutionCompatibilityOracle(ctx, stateRoot, record, req, deps, workspaceReq, probe, result)
}

func prepareDirectExecutionCompatibilityOracle(ctx context.Context, stateRoot string, record IssueOpsRecord, req ExecutionPrepareRequest, deps ExecutionPrepareDependencies, workspaceReq port.ExecutionWorkspaceRequest, result ExecutionPrepareResult) (ExecutionPrepareResult, error) {
	if deps.Direct == nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("direct Git worktree provisioner is unavailable")
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	if !samePath(req.CWD, record.Repo) && !samePath(req.CWD, workspaceReq.Root) {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("direct prepare cwd must be source_root or the canonical worktree")
	}
	if req.Confirm {
		prober, ok := deps.Direct.(port.ExecutionWorkspaceAccessProber)
		if !ok {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("direct provisioner cannot verify canonical worktree base access")
		}
		access, err := prober.ProbeAccess(ctx, workspaceReq, actor.Host)
		if err != nil {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, err
		}
		if !access.Allowed {
			return ExecutionPrepareResult{
				OK: false, ID: record.ID, RequestedMode: result.RequestedMode, ResolvedMode: result.ResolvedMode,
				Workspace: result.Workspace, NextCommand: access.RelaunchCommand,
			}, fmt.Errorf("canonical worktree base is not accessible; relaunch with: %s", access.RelaunchCommand)
		}
	}
	receipt, err := deps.Direct.Prepare(ctx, workspaceReq)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	result.Workspace = workspaceFromReceipt(receipt, executionNow(deps.Now))
	if !req.Confirm {
		return result, nil
	}
	record.WorktreePath = receipt.Root
	record.Execution = &model.Execution{
		Mode: model.ExecutionModeDirect, Workspace: result.Workspace,
		Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &actor, ClaimedAt: executionNow(deps.Now)},
	}
	if _, err := materializeStagedArtifacts(stateRoot, record); err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return err
		}
		if current.Execution != nil {
			persisted = current
			return nil
		}
		if err := ensureExecutionRootUnclaimed(stateRoot, current.ID, record.WorktreePath); err != nil {
			return err
		}
		current.WorktreePath = record.WorktreePath
		current.Execution = record.Execution
		persisted, err = persistExecutionTransition(stateRoot, current, nil)
		return err
	})
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	return preparedExecutionResultWithModes(persisted, result.RequestedMode, result.FallbackCode), nil
}

func prepareOrcaExecutionCompatibilityOracle(ctx context.Context, stateRoot string, record IssueOpsRecord, req ExecutionPrepareRequest, deps ExecutionPrepareDependencies, workspaceReq port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest, result ExecutionPrepareResult) (ExecutionPrepareResult, error) {
	if deps.Orca == nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("Orca provisioner is unavailable")
	}
	if req.Confirm {
		actor, err := normalizeNativeActor(req.Actor)
		if err != nil {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, err
		}
		req.Actor = actor
	}
	fromParentWorktree := strings.TrimSpace(workspaceReq.ParentWorktree) != "" && samePath(req.CWD, workspaceReq.ParentWorktree)
	if !samePath(req.CWD, record.Repo) && !samePath(req.CWD, workspaceReq.Root) && !fromParentWorktree {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("Orca prepare cwd must be source_root, the canonical worktree, or the sealed parent worktree")
	}
	snapshot, err := readExecutionOwnerSnapshot(ctx, record, deps.ReadIssue)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	if !req.Confirm {
		return result, nil
	}
	pending, payload, err := beginOrcaExecutionIntentWithID(stateRoot, record, workspaceReq, probe, req, snapshot, deps.OperationID, deps.Now)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	for step := 0; pending.Execution != nil && pending.Execution.Pending != nil; step++ {
		if step >= 6 {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("Orca prepare exceeded the fixed external intent stage count")
		}
		pending, payload, err = executeOrcaIntentStage(ctx, stateRoot, pending, payload, deps.Orca, deps.ReadIssue, deps.Now)
		if err != nil {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, err
		}
	}
	result = preparedExecutionResultWithModes(pending, result.RequestedMode, result.FallbackCode)
	result.ClaimTokenPath = claimTokenPath(pending)
	result.IssueBodySHA256 = snapshot.issue.BodySHA256
	if payload.Launch != nil {
		result.ContextPacketPath, result.ContextPacketSHA256 = payload.Launch.ContextPacketPath, payload.Launch.ContextPacketSHA256
		result.OwnerPromptPath, result.OwnerPromptSHA256 = payload.Launch.PromptPath, payload.Launch.PromptSHA256
	}
	return result, nil
}
