package issueopslease

import (
	"context"

	leaseapp "agent-harness/internal/application/issueopslease"
	"agent-harness/internal/core/issueops"
)

type ReconcileHandler struct{ service *leaseapp.ReconcileService }

func NewReconcileHandler(service *leaseapp.ReconcileService) issueops.ExecutionReconcileHandler {
	return ReconcileHandler{service: service}.Handle
}

func (h ReconcileHandler) Handle(ctx context.Context, _ string, request issueops.ExecutionReconcileRequest, _ issueops.ExecutionReconcileDependencies) (issueops.ExecutionReconcileResult, error) {
	if h.service == nil {
		return issueops.ExecutionReconcileResult{ID: request.ID}, issueops.ErrReconcileHandlerUnavailable
	}
	result, err := h.service.Reconcile(ctx, leaseapp.ReconcileRequest{ID: request.ID})
	public := issueops.ExecutionReconcileResult{
		OK: result.OK, ID: result.ID, Reconciled: result.Reconciled, Code: result.Code,
		ExternalStateInspected: result.ExternalStateInspected,
	}
	if result.IntentMigrated {
		public.IntentMigrationCode = "legacy_intent_upgraded"
	}
	if result.Record.Execution != nil {
		public.Execution = toCoreExecution(*result.Record.Execution)
		public.Pending = public.Execution.Pending
	}
	return public, err
}
