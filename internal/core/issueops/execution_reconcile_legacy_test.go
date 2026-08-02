package issueops

import "context"

// predecessor 회귀 픽스처는 shared durable primitive 자체를 계속 검증한다.
// production wiring은 이 test-only oracle이 아니라 새 vertical을 사용한다.
func legacyReconcileTestHandler(ctx context.Context, stateRoot string, req ExecutionReconcileRequest, deps ExecutionReconcileDependencies) (result ExecutionReconcileResult, err error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionReconcileResult{ID: req.ID}, err
	}
	record, payload, migrated, err := reconcileCanonicalOrcaIntent(stateRoot, record)
	if err != nil {
		return failedExecutionReconcileResult(record, "legacy_intent_upgrade_unsafe"), err
	}
	updated, next, err := executeOrcaIntentStage(ctx, stateRoot, record, payload, deps.Orca, deps.ReadIssue, deps.Now)
	if err != nil {
		if latest, readErr := ReadIssueOps(stateRoot, record.ID); readErr == nil {
			updated = latest
		}
		result = failedExecutionReconcileResult(updated, "orca_reconcile_ambiguous")
		result.ExternalStateInspected = true
		if migrated {
			result.IntentMigrationCode = "legacy_intent_upgraded"
		}
		return result, err
	}
	code := "orca_reconcile_completed"
	if updated.Execution != nil && updated.Execution.Pending != nil {
		code = "orca_reconcile_advanced_" + string(next.Stage)
	}
	result = executionReconcileResult(updated, false, code)
	result.Reconciled = true
	result.ExternalStateInspected = true
	if migrated {
		result.IntentMigrationCode = "legacy_intent_upgraded"
	}
	return result, nil
}
