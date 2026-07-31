package issueopslease

import (
	"context"
	"errors"
	"fmt"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

type ResumeRequest struct {
	ID                 string
	ExpectedGeneration uint64
	Actor              leasedomain.Actor
	Ancestry           []leasedomain.ProcessReceipt
	CWD                string
	Confirm            bool
}

type ResumeResult struct {
	OK      bool
	ID      string
	Receipt leasecontract.ResumeReceipt
}

type ResumeService struct {
	fence        ResumeFence
	repository   ResumeRepository
	artifacts    ResumeArtifacts
	owners       ResumeOwnerInventory
	stages       ResumeStageExecutor
	operationIDs ResumeOperationIDs
	inspect      ProcessInspector
	paths        CanonicalPathMatcher
}

func NewResumeService(fence ResumeFence, repository ResumeRepository, artifacts ResumeArtifacts, owners ResumeOwnerInventory, stages ResumeStageExecutor, operationIDs ResumeOperationIDs, inspect ProcessInspector, paths CanonicalPathMatcher) *ResumeService {
	return &ResumeService{fence: fence, repository: repository, artifacts: artifacts, owners: owners, stages: stages, operationIDs: operationIDs, inspect: inspect, paths: paths}
}

func (s *ResumeService) Resume(ctx context.Context, request ResumeRequest) (ResumeResult, error) {
	if !request.Confirm {
		return ResumeResult{ID: request.ID}, fmt.Errorf("execution resume requires confirm")
	}
	if s.fence == nil || s.repository == nil || s.artifacts == nil || s.owners == nil || s.stages == nil || s.operationIDs == nil || s.paths == nil {
		return ResumeResult{ID: request.ID}, fmt.Errorf("resume service dependencies are required")
	}
	var result ResumeResult
	err := s.fence.Within(ctx, request.ID, func(fenceCtx context.Context) error {
		if _, err := resolveActor(fenceCtx, request.Actor, request.Ancestry, s.inspect); err != nil {
			return err
		}
		snapshot, err := s.repository.LoadSnapshot(fenceCtx, request.ID, request.ExpectedGeneration)
		if err != nil {
			return err
		}
		if snapshot.Record.Stable.Execution == nil || snapshot.Record.Stable.Execution.Orca == nil {
			return fmt.Errorf("execution resume requires an existing Orca binding")
		}
		preflight := resumeDomainRequest(snapshot.Record.Stable, request.ExpectedGeneration, s.paths.Matches(request.CWD, snapshot.Record.CanonicalRoot), leasedomain.ResumeInventory{}, true)
		if _, err := leasedomain.PlanResume(preflight); err != nil {
			return err
		}
		artifacts, err := s.artifacts.ReadAndVerify(fenceCtx, snapshot.Record.Stable)
		if err != nil {
			return err
		}
		inventory, runtimeCompatible, err := s.owners.Observe(fenceCtx, snapshot.Record.Stable)
		if err != nil {
			return err
		}
		plan, err := leasedomain.PlanResume(resumeDomainRequest(snapshot.Record.Stable, request.ExpectedGeneration, true, inventory, runtimeCompatible))
		if err != nil {
			return err
		}
		if plan.Disposition == leasedomain.ResumeExistingBinding {
			result = ResumeResult{OK: true, ID: request.ID, Receipt: leasecontract.ResumeReceipt{Execution: *snapshot.Record.Stable.Execution, Artifacts: artifacts}}
			return nil
		}
		operationID, err := s.operationIDs.New()
		if err != nil {
			return err
		}
		progress, err := s.repository.BeginIntent(fenceCtx, snapshot, artifacts, plan, operationID)
		if err != nil {
			return err
		}
		for attempts := 0; attempts < 5 && progress.Pending; attempts++ {
			intent, err := s.repository.LoadIntent(fenceCtx, progress)
			if err != nil {
				return err
			}
			inventory, err := s.stages.Inspect(fenceCtx, intent)
			if err != nil {
				cause := fmt.Errorf("Orca intent inventory is ambiguous; intent retained: %w", err)
				_ = s.repository.RecordFailure(fenceCtx, intent, intent.InvocationState, err)
				return cause
			}
			stagePlan, err := leasedomain.PlanResumeStage(leasedomain.ResumeStageRequest{CandidateCount: len(inventory.Candidates), AuthoritativeZero: inventory.AuthoritativeZero, InvocationState: intent.InvocationState, InvocationAttempts: intent.InvocationAttempts})
			if err != nil {
				cause := resumeStageDecisionCause(intent, inventory, err)
				_ = s.repository.RecordFailure(fenceCtx, intent, intent.InvocationState, cause)
				return cause
			}
			var receipt leasecontract.ResumeStageReceipt
			receiptIntent := intent
			switch stagePlan.Action {
			case leasedomain.ResumeStageAdopt:
				receipt = inventory.Candidates[stagePlan.CandidateIndex]
			case leasedomain.ResumeStageInvoke:
				receiptIntent, err = s.repository.MarkInvoking(fenceCtx, intent)
				if err != nil {
					return err
				}
				receipt, err = s.stages.Invoke(fenceCtx, receiptIntent)
				if err != nil {
					_ = s.repository.RecordFailure(fenceCtx, receiptIntent, resumeInvocationFailureState(err), err)
					return resumeReconcileRequired(err)
				}
			case leasedomain.ResumeStageReconcile:
				cause := resumeStageReconcileCause(stagePlan.Reason)
				_ = s.repository.RecordFailure(fenceCtx, intent, intent.InvocationState, cause)
				return cause
			}
			progress, err = s.repository.ApplyReceipt(fenceCtx, receiptIntent, receipt)
			if err != nil {
				_ = s.repository.RecordFailure(fenceCtx, receiptIntent, "unknown", err)
				return err
			}
		}
		if progress.Pending {
			return fmt.Errorf("execution resume did not complete the owner launch stages")
		}
		result = ResumeResult{OK: true, ID: request.ID, Receipt: leasecontract.ResumeReceipt{Execution: progress.Execution, Artifacts: artifacts}}
		return nil
	})
	if err != nil {
		return ResumeResult{ID: request.ID}, err
	}
	return result, nil
}

func resumeDomainRequest(record leasecontract.Record, generation uint64, canonicalCWD bool, inventory leasedomain.ResumeInventory, runtimeCompatible bool) leasedomain.ResumeRequest {
	binding := record.Execution.Orca
	return leasedomain.ResumeRequest{
		ExpectedGeneration: generation, Lease: toDomainLease(record.Execution.Lease), BindingGeneration: binding.LeaseGeneration,
		BindingRuntimeID: binding.RuntimeID, BindingTerminalID: binding.TerminalPTYID, CanonicalCWD: canonicalCWD,
		ModeOrca: record.Execution.Mode == "orca", BindingPresent: binding != nil, PendingAbsent: record.Execution.Pending == nil,
		RuntimeCompatible: runtimeCompatible, Inventory: inventory,
	}
}

func resumeReconcileRequired(cause error) error {
	return fmt.Errorf("Orca mutation outcome requires execution reconcile; mutation was not repeated: %w", cause)
}

func resumeStageDecisionCause(intent ResumeIntentState, inventory leasecontract.ResumeStageInventory, fallback error) error {
	if len(inventory.Candidates) > 1 {
		return fmt.Errorf("Orca intent inventory found multiple candidates; intent retained")
	}
	if intent.InvocationAttempts >= 2 {
		return fmt.Errorf("Orca intent retry is exhausted; intent retained")
	}
	return fallback
}

func resumeStageReconcileCause(reason string) error {
	switch reason {
	case "non-authoritative-zero":
		return fmt.Errorf("Orca intent inventory returned a non-authoritative zero; intent retained")
	case "unknown-invocation":
		return fmt.Errorf("authoritative zero cannot retry an Orca mutation whose absence was not proven; intent retained")
	default:
		return fmt.Errorf("Orca intent inventory %s; intent retained", reason)
	}
}

func resumeInvocationFailureState(err error) string {
	var typed *port.OrcaError
	if errors.As(err, &typed) && !typed.Invoked {
		return "not_invoked_proven"
	}
	return "unknown"
}
