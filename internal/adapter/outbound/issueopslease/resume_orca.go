package issueopslease

import (
	"context"
	"fmt"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type ResumeOwnerObserver func(context.Context, leasecontract.Record) (leasedomain.ResumeInventory, error)

type ResumeOwnerInventory struct{ observe ResumeOwnerObserver }

func NewResumeOwnerInventory(observe ResumeOwnerObserver) *ResumeOwnerInventory {
	return &ResumeOwnerInventory{observe: observe}
}

func (a *ResumeOwnerInventory) Observe(ctx context.Context, record leasecontract.Record) (leasedomain.ResumeInventory, error) {
	if a == nil || a.observe == nil {
		return leasedomain.ResumeInventory{}, fmt.Errorf("resume owner observer is required")
	}
	return a.observe(ctx, record)
}

type ResumeStageInspector func(context.Context, leaseapp.ResumeIntentState) (leasecontract.ResumeStageInventory, error)
type ResumeStageInvoker func(context.Context, leaseapp.ResumeIntentState) (leasecontract.ResumeStageReceipt, error)

type ResumeStageExecutor struct {
	inspect ResumeStageInspector
	invoke  ResumeStageInvoker
}

func NewResumeStageExecutor(inspect ResumeStageInspector, invoke ResumeStageInvoker) *ResumeStageExecutor {
	return &ResumeStageExecutor{inspect: inspect, invoke: invoke}
}

func (a *ResumeStageExecutor) Inspect(ctx context.Context, intent leaseapp.ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
	if a == nil || a.inspect == nil {
		return leasecontract.ResumeStageInventory{}, fmt.Errorf("resume stage inspector is required")
	}
	return a.inspect(ctx, intent)
}

func (a *ResumeStageExecutor) Invoke(ctx context.Context, intent leaseapp.ResumeIntentState) (leasecontract.ResumeStageReceipt, error) {
	if a == nil || a.invoke == nil {
		return leasecontract.ResumeStageReceipt{}, fmt.Errorf("resume stage invoker is required")
	}
	return a.invoke(ctx, intent)
}
