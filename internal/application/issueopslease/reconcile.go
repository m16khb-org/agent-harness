package issueopslease

import (
	"context"
	"fmt"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

type ReconcileRequest struct {
	ID string
}

type ReconcileResult struct {
	OK                     bool
	ID                     string
	Record                 leasecontract.Record
	Reconciled             bool
	Code                   string
	ExternalStateInspected bool
	IntentMigrated         bool
}

type ReconcileService struct {
	repository ReconcileRepository
	stages     ReconcileStageExecutor
}

func NewReconcileService(repository ReconcileRepository, stages ReconcileStageExecutor) *ReconcileService {
	return &ReconcileService{repository: repository, stages: stages}
}

func (s *ReconcileService) Reconcile(ctx context.Context, request ReconcileRequest) (ReconcileResult, error) {
	base := ReconcileResult{ID: request.ID}
	if s == nil || s.repository == nil || s.stages == nil {
		return base, fmt.Errorf("reconcile service dependencies are required")
	}
	intent, err := s.repository.Canonicalize(ctx, request.ID)
	base.Record = intent.Progress.Record
	base.IntentMigrated = intent.Migrated
	if err != nil {
		base.Code = "legacy_intent_upgrade_unsafe"
		return base, err
	}
	inventory, attempted, err := s.stages.Inspect(ctx, intent)
	base.ExternalStateInspected = attempted
	if err != nil {
		if attempted {
			_ = s.repository.RecordFailure(ctx, intent, intent.InvocationState, err)
			err = fmt.Errorf("Orca intent inventory is ambiguous; intent retained: %w", err)
		}
		return s.failed(ctx, base, err)
	}
	plan, err := leasedomain.PlanReconcileStage(leasedomain.ReconcileStageRequest{
		Stage: intent.Stage, CandidateCount: len(inventory.Candidates), AuthoritativeZero: inventory.AuthoritativeZero,
		InvocationState: intent.InvocationState, InvocationAttempts: intent.InvocationAttempts,
	})
	if err != nil {
		return s.failed(ctx, base, err)
	}
	receiptIntent, receipt, err := s.executePlan(ctx, intent, inventory, plan)
	if err != nil {
		return s.failed(ctx, base, err)
	}
	progress, err := s.repository.ApplyReceipt(ctx, receiptIntent, receipt)
	if err != nil {
		_ = s.repository.RecordFailure(ctx, receiptIntent, "unknown", err)
		return s.failed(ctx, base, err)
	}
	base.OK = true
	base.Record = progress.Record
	base.Reconciled = true
	base.Code = "orca_reconcile_completed"
	if progress.Pending {
		base.Code = "orca_reconcile_advanced_" + progress.NextStage
	}
	return base, nil
}

func (s *ReconcileService) executePlan(ctx context.Context, intent ReconcileIntentState, inventory leasecontract.ReconcileStageInventory, plan leasedomain.ReconcileStagePlan) (ReconcileIntentState, leasecontract.ReconcileStageReceipt, error) {
	switch plan.Action {
	case leasedomain.ReconcileStageAdopt:
		return intent, inventory.Candidates[plan.CandidateIndex], nil
	case leasedomain.ReconcileStageInvoke:
		invoking, err := s.repository.MarkInvoking(ctx, intent)
		if err != nil {
			return intent, leasecontract.ReconcileStageReceipt{}, err
		}
		receipt, failureState, err := s.stages.Invoke(ctx, invoking)
		if err == nil {
			return invoking, receipt, nil
		}
		if failureState == "" {
			failureState = "unknown"
		}
		_ = s.repository.RecordFailure(ctx, invoking, failureState, err)
		return invoking, leasecontract.ReconcileStageReceipt{}, fmt.Errorf("Orca mutation outcome requires execution reconcile; mutation was not repeated: %w", err)
	case leasedomain.ReconcileStagePreserve:
		cause := reconcilePreserveCause(plan.Reason)
		_ = s.repository.RecordFailure(ctx, intent, intent.InvocationState, cause)
		return intent, leasecontract.ReconcileStageReceipt{}, cause
	default:
		return intent, leasecontract.ReconcileStageReceipt{}, fmt.Errorf("unsupported reconcile stage action %q", plan.Action)
	}
}

func (s *ReconcileService) failed(ctx context.Context, result ReconcileResult, cause error) (ReconcileResult, error) {
	result.Code = "orca_reconcile_ambiguous"
	if latest, err := s.repository.Latest(ctx, result.ID); err == nil {
		result.Record = latest
	}
	return result, cause
}

func reconcilePreserveCause(reason string) error {
	switch reason {
	case "multiple-candidates":
		return fmt.Errorf("Orca intent inventory found multiple candidates; intent retained")
	case "non-authoritative-zero":
		return fmt.Errorf("Orca intent inventory returned a non-authoritative zero; intent retained")
	case "unknown-invocation":
		return fmt.Errorf("authoritative zero cannot retry an Orca mutation whose absence was not proven; intent retained")
	case "retry-exhausted":
		return fmt.Errorf("Orca intent retry is exhausted; intent retained")
	default:
		return fmt.Errorf("Orca intent inventory %s; intent retained", reason)
	}
}
