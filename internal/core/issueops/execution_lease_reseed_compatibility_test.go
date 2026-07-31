package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
)

func reseedExecutionCompatibilityOracle(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if req.Action != ExecutionReplaceReseed {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("compatibility oracle supports only reseed")
	}
	if !req.Confirm {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("reseed requires confirm")
	}
	var persisted IssueOpsRecord
	var tokenPath string
	var resealed executionOwnerReseal
	err = withIssueOpsLock(ctx, stateRoot, req.ID, func(context.Context) error {
		record, err := executionRecordAtGeneration(stateRoot, req.ID, req.ExpectedGeneration)
		if err != nil {
			return err
		}
		if record.Execution.Pending != nil {
			return fmt.Errorf("execution replacement is blocked by a pending external intent; run execution reconcile")
		}
		if err := validateExecutionReplacementCWD(record, req.CWD); err != nil {
			return err
		}
		lease := &record.Execution.Lease
		if lease.Status != model.LeaseStatusReleased && lease.Status != model.LeaseStatusClaimable {
			return fmt.Errorf("reseed requires a released or claimable lease")
		}
		fingerprint, orcaInventory, err := executionInventoryFingerprint(ctx, record, actor, deps)
		if err != nil {
			return err
		}
		if fingerprint != req.InventoryFingerprint {
			return fmt.Errorf("stale replacement inventory fingerprint")
		}
		if record.Execution.Orca != nil && strings.TrimSpace(orcaInventory.RuntimeID) != "" {
			record.Execution.Orca.RuntimeID = orcaInventory.RuntimeID
		}
		supersededTokenPath := claimTokenPath(record)
		lease.Generation++
		if err := cleanupReplacementGeneration(record); err != nil {
			return err
		}
		token, path, err := createClaimToken(record)
		if err != nil {
			return cleanupReplacementFailure(record, err)
		}
		tokenPath = path
		lease.Status = model.LeaseStatusClaimable
		lease.Holder = nil
		lease.ClaimTokenSHA256 = tokenSHA256(token)
		lease.ReplacedAt = time.Now().UTC().Format(time.RFC3339Nano)
		lease.ReplacementReason = strings.TrimSpace(req.Reason)
		reseal, err := resealOwnerContextForReplacement(ctx, stateRoot, record, deps)
		if err != nil {
			return cleanupReplacementFailure(record, err)
		}
		resealed = reseal
		persisted, err = persistExecutionTransition(stateRoot, record, nil)
		if err != nil {
			return cleanupReplacementFailure(record, err)
		}
		_ = removeReplacementRuntimeFile(record.Execution.Workspace.Root, supersededTokenPath)
		return nil
	})
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	result := replaceResult(persisted, req.Action, "", "", tokenPath)
	result.IssueBodySHA256 = resealed.issueBodySHA256
	result.ContextPacketPath, result.ContextPacketSHA256 = resealed.packetPath, resealed.packetSHA256
	result.OwnerPromptPath, result.OwnerPromptSHA256 = resealed.promptPath, resealed.promptSHA256
	if persisted.Execution.Lease.Status == model.LeaseStatusClaimable {
		result.NextCommand = ExecutionReseedNextCommand(
			persisted.ID, persisted.Execution.Lease.Generation, string(persisted.Execution.Mode), tokenPath,
		)
	}
	return result, nil
}
