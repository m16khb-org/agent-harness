package issueopspublication

import (
	"context"
	"encoding/json"
	"fmt"

	publicationapp "agent-harness/internal/application/issueopspublication"
	publicationcontract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/core/issueops"
)

type reconcileService interface {
	Reconcile(context.Context, string) (publicationcontract.ReconcileResult, error)
}

var _ reconcileService = (*publicationapp.ReconcileService)(nil)

type ReconcileHandler struct{ service reconcileService }

func NewReconcileHandler(service reconcileService) issueops.RemotePullRequestReconcileHandler {
	return ReconcileHandler{service: service}.Handle
}

func (h ReconcileHandler) Handle(ctx context.Context, _ string, request issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
	if h.service == nil {
		return issueops.ExecutionReconcileResult{ID: request.ID}, issueops.ErrRemotePullRequestReconcileHandlerUnavailable
	}
	result, serviceErr := h.service.Reconcile(ctx, request.ID)
	public := issueops.ExecutionReconcileResult{
		OK: serviceErr == nil || result.Reconciled, ID: request.ID, Reconciled: result.Reconciled, Code: result.Code,
		ExternalStateInspected: result.ExternalStateInspected,
	}
	if result.Record.ID != "" {
		public.ID = result.Record.ID
	}
	var record issueops.IssueOpsRecord
	if serviceErr != nil && !result.Reconciled && request.Snapshot != nil {
		record = *request.Snapshot
	} else {
		if len(result.Record.Raw) == 0 {
			if serviceErr != nil {
				return public, serviceErr
			}
			return public, fmt.Errorf("decode publication record snapshot: raw record is required")
		}
		if err := json.Unmarshal(result.Record.Raw, &record); err != nil {
			if serviceErr != nil {
				return public, serviceErr
			}
			return public, fmt.Errorf("decode publication record snapshot: %w", err)
		}
	}
	if record.Execution != nil {
		public.Execution = *record.Execution
		public.Pending = public.Execution.Pending
	}
	return public, serviceErr
}
