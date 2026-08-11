package issueopsstatus

import issueopsstatuscontract "agent-harness/internal/contract/issueopsstatus"

type Completion func(
	issueopsstatuscontract.Record,
	issueopsstatuscontract.Phase,
) issueopsstatuscontract.Readiness

type Projector struct {
	completion Completion
}

func NewProjector(completion Completion) Projector {
	return Projector{completion: completion}
}

func (projector Projector) Project(record issueopsstatuscontract.Record) issueopsstatuscontract.Record {
	derived := projector.Derive(record)
	if len(record.PhaseLedger) == 0 {
		record.PhaseLedger = derived
		return record
	}
	for phase, entry := range derived {
		if _, exists := record.PhaseLedger[phase]; !exists {
			record.PhaseLedger[phase] = entry
		}
	}
	return record
}

func (projector Projector) Derive(record issueopsstatuscontract.Record) issueopsstatuscontract.PhaseLedger {
	ledger := issueopsstatuscontract.PhaseLedger{}
	for _, phase := range [...]issueopsstatuscontract.Phase{
		issueopsstatuscontract.PhaseProblem,
		issueopsstatuscontract.PhaseGrill,
		issueopsstatuscontract.PhasePlan,
		issueopsstatuscontract.PhaseCompatibilityReview,
		issueopsstatuscontract.PhaseImplement,
		issueopsstatuscontract.PhaseAISlopClean,
		issueopsstatuscontract.PhaseFeedback,
		issueopsstatuscontract.PhasePR,
		issueopsstatuscontract.PhaseDone,
	} {
		entry := issueopsstatuscontract.PhaseLedgerEntry{
			Phase: phase,
			Notes: []string{"derived"},
		}
		completion := projector.completion(record, phase)
		if completion.Ready {
			entry.CompletedAt = ""
			entry.Artifacts = ArtifactKeys(phase)
		} else {
			entry.Missing = completion.Missing
		}
		ledger[phase] = entry
	}
	return ledger
}

func ArtifactKeys(phase issueopsstatuscontract.Phase) []string {
	switch phase {
	case issueopsstatuscontract.PhaseProblem:
		return []string{"intent_contract"}
	case issueopsstatuscontract.PhaseGrill:
		return []string{"issue_url", "branch", "plan_prep", "split_decision", "domain_review"}
	case issueopsstatuscontract.PhasePlan:
		return []string{"branch_prepare", "worktree_path", "plan_path", "design_review"}
	case issueopsstatuscontract.PhaseCompatibilityReview:
		return []string{"compatibility_review", "compatibility_approval", "compatibility_blockers"}
	case issueopsstatuscontract.PhaseImplement:
		return []string{"worktree_tools", "execution_decision", "implementation_changes"}
	case issueopsstatuscontract.PhaseAISlopClean:
		return []string{"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint", "cleanup_evidence", "verification_evidence"}
	case issueopsstatuscontract.PhaseFeedback:
		return []string{"feedback_classification", "contract_feedback_issue_update", "feedback_resolution"}
	case issueopsstatuscontract.PhasePR:
		return []string{"strict_pr_readiness", "children_complete", "remote_artifact", "target_branch_match"}
	case issueopsstatuscontract.PhaseDone:
		return []string{"prior_phase_pr", "verified_remote_artifact"}
	default:
		return nil
	}
}
