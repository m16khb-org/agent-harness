package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type ExecutionResumeDependencies struct {
	Orca        port.ExecutionOrcaProvisioner
	OrcaOwner   port.ExecutionOrcaOwnerInspector
	Now         func() time.Time
	OperationID string
}

func ResumeExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionResumeRequest, deps ExecutionResumeDependencies) (ExecutionResumeResult, error) {
	if !req.Confirm {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires confirm")
	}
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	if _, err := normalizeNativeActor(req.Actor); err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	record, err := executionRecordAtGeneration(stateRoot, req.ID, req.ExpectedGeneration)
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	if record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Orca == nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires an existing Orca binding")
	}
	if record.Execution.Pending != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume is blocked by a pending external intent; run execution reconcile")
	}
	if record.Execution.Lease.Status != model.LeaseStatusClaimable || record.Execution.Lease.Holder != nil || !executionSHA256.MatchString(record.Execution.Lease.ClaimTokenSHA256) {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires a holderless claimable lease")
	}
	if !samePath(req.CWD, record.Execution.Workspace.Root) {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume cwd must be the canonical worktree")
	}
	artifacts, err := readExecutionResumeArtifacts(record)
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	if deps.Orca == nil || deps.OrcaOwner == nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires Orca mutation and owner inventory adapters")
	}
	binding := record.Execution.Orca
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID, AllowRuntimeRollover: true})
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("inspect previous Orca owner: %w", err)
	}
	if err := validateExecutionRuntimeRollover(record, inventory); err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	sameGeneration := binding.LeaseGeneration == record.Execution.Lease.Generation
	if inventory.TaskLive && !inventory.TerminalLive {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("Orca owner inventory has a live task without a live terminal")
	}
	if inventory.TaskLive {
		if !sameGeneration {
			return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("previous Orca owner task is still live")
		}
		return executionResumeResult(record, artifacts), nil
	}
	reusedTerminalPTYID := ""
	if inventory.TerminalLive {
		reusedTerminalPTYID = strings.TrimSpace(inventory.TerminalID)
		if reusedTerminalPTYID == "" || reusedTerminalPTYID != strings.TrimSpace(binding.TerminalPTYID) {
			return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("Orca owner terminal identity changed")
		}
	}
	runtimeID := strings.TrimSpace(inventory.RuntimeID)
	if runtimeID == "" {
		runtimeID = binding.RuntimeID
	}
	var persisted IssueOpsRecord
	var payload externalOrcaIntentPayload
	if deps.OperationID == "" {
		persisted, payload, err = beginOrcaExecutionResumeIntent(stateRoot, record, artifacts, runtimeID, reusedTerminalPTYID, deps.Now)
	} else {
		persisted, payload, err = beginOrcaExecutionResumeIntentWithID(stateRoot, record, artifacts, runtimeID, reusedTerminalPTYID, deps.OperationID, deps.Now)
	}
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	for attempt := 0; attempt < 5 && persisted.Execution.Pending != nil; attempt++ {
		persisted, payload, err = executeOrcaIntentStage(ctx, stateRoot, persisted, payload, deps.Orca, nil, deps.Now)
		if err != nil {
			return ExecutionResumeResult{OK: false, ID: req.ID}, err
		}
	}
	if persisted.Execution.Pending != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume did not complete the owner launch stages")
	}
	return executionResumeResult(persisted, artifacts), nil
}
