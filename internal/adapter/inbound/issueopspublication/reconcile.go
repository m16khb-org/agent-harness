package issueopspublication

import (
	"context"
	"encoding/json"
	"fmt"

	issueopscontract "issueops/internal/contract/issueops"

	publicationapp "issueops/internal/application/issueopspublication"
	publicationcontract "issueops/internal/contract/issueopspublication"
)

type reconcileService interface {
	Reconcile(context.Context, string) (publicationcontract.ReconcileResult, error)
}

var _ reconcileService = (*publicationapp.ReconcileService)(nil)

type ReconcileHandler struct{ service reconcileService }

// 반환 타입은 어댑터의 이름 붙은 핸들러 타입 대신 같은 시그니처를 직접 쓴다.
func NewReconcileHandler(service reconcileService) func(context.Context, string, issueopscontract.ExecutionReconcileRequest) (issueopscontract.ExecutionReconcileResult, error) {
	return ReconcileHandler{service: service}.Handle
}

func (h ReconcileHandler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionReconcileRequest) (issueopscontract.ExecutionReconcileResult, error) {
	if h.service == nil {
		return issueopscontract.ExecutionReconcileResult{ID: request.ID}, issueopscontract.ErrRemotePullRequestReconcileHandlerUnavailable
	}
	result, serviceErr := h.service.Reconcile(ctx, request.ID)
	public := issueopscontract.ExecutionReconcileResult{
		OK: serviceErr == nil || result.Reconciled, ID: request.ID, Reconciled: result.Reconciled, Code: result.Code,
		ExternalStateInspected: result.ExternalStateInspected,
	}
	if result.Record.ID != "" {
		public.ID = result.Record.ID
	}
	var record issueopscontract.IssueOpsRecord
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
