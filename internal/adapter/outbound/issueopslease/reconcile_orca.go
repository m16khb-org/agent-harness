package issueopslease

import (
	"context"
	"fmt"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
)

type ReconcileStageInspector func(context.Context, leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error)
type ReconcileStageInvoker func(context.Context, leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageReceipt, string, error)

type ReconcileStageExecutor struct {
	inspect ReconcileStageInspector
	invoke  ReconcileStageInvoker
}

func NewReconcileStageExecutor(inspect ReconcileStageInspector, invoke ReconcileStageInvoker) *ReconcileStageExecutor {
	return &ReconcileStageExecutor{inspect: inspect, invoke: invoke}
}

func (a *ReconcileStageExecutor) Inspect(ctx context.Context, intent leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
	if a == nil || a.inspect == nil {
		return leasecontract.ReconcileStageInventory{}, false, fmt.Errorf("reconcile stage inspector is required")
	}
	return a.inspect(ctx, intent)
}

func (a *ReconcileStageExecutor) Invoke(ctx context.Context, intent leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageReceipt, string, error) {
	if a == nil || a.invoke == nil {
		return leasecontract.ReconcileStageReceipt{}, "unknown", fmt.Errorf("reconcile stage invoker is required")
	}
	return a.invoke(ctx, intent)
}
