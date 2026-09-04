package issueopslease

import (
	"context"
	"errors"
	"testing"

	"issueops/internal/adapter/issueops"
)

func TestReconcileHandlerFailsClosedWithoutService(t *testing.T) {
	result, err := NewReconcileHandler(nil)(context.Background(), "state", issueops.ExecutionReconcileRequest{ID: "io-reconcile"}, issueops.ExecutionReconcileDependencies{})
	if !errors.Is(err, issueops.ErrReconcileHandlerUnavailable) || result.ID != "io-reconcile" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
