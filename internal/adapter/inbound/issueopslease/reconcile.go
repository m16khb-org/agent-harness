package issueopslease

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
	"context"

	leaseapp "agent-harness/internal/application/issueopslease"
)

type ReconcileHandler struct{ service *leaseapp.ReconcileService }

func NewReconcileHandler(service *leaseapp.ReconcileService) port.ExecutionReconcileHandler {
	return ReconcileHandler{service: service}.Handle
}

func (h ReconcileHandler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionReconcileRequest, _ port.ExecutionReconcileDependencies) (issueopscontract.ExecutionReconcileResult, error) {
	if h.service == nil {
		return issueopscontract.ExecutionReconcileResult{ID: request.ID}, issueopscontract.ErrReconcileHandlerUnavailable
	}
	result, err := h.service.Reconcile(ctx, leaseapp.ReconcileRequest{ID: request.ID})
	public := issueopscontract.ExecutionReconcileResult{
		OK: result.OK, ID: result.ID, Reconciled: result.Reconciled, Code: result.Code,
		ExternalStateInspected: result.ExternalStateInspected,
	}
	if result.Record.Execution != nil {
		public.Execution = toCoreExecution(*result.Record.Execution)
		public.Pending = public.Execution.Pending
	}
	return public, err
}
