package issueops

import (
	"strings"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/stringlist"
)

// issueOpsLedgerDerivedSentinel is written for phase timestamps that were not
// observed live (derived/backfilled from existing fields). It is deliberately
// empty so derivation never fabricates wall-clock precision and stays
// byte-deterministic across hosts and runs.
const issueOpsLedgerDerivedSentinel = ""

func issueOpsReadinessFrom(record IssueOpsRecord, missing []string) IssueOpsReadiness {
	return IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

// IssueOpsProblemReadiness reports whether the problem phase is complete.
// Problem completion is intentionally minimal — only the intent contract — so
// the free problem -> grill step and early exploration before the remote issue
// or branch exist are preserved. issue_url/branch are grill artifacts.
func IssueOpsProblemReadiness(record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsReadinessFrom(record, issueOpsIntentMissing(record))
}

// IssueOpsGrillReadiness reports whether the grill phase is complete:
// issue_url + branch + plan_prep (when the gate applies) + split_decision +
// domain_review. These gate plan entry, matching the create-issue-after-grill
// workflow and today's plan-entry gate.
func IssueOpsGrillReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.Branch) == "" {
		missing = append(missing, "branch")
	}
	if planPrepGateApplies(record) {
		missing = append(missing, planPrepMissing(record.PlanPrep)...)
	}
	missing = append(missing, issueOpsSplitDecisionMissing(record)...)
	missing = append(missing, issueOpsDomainReviewMissing(record)...)
	return issueOpsReadinessFrom(record, missing)
}

// issueOpsSplitDecisionMissing derives split_decision from existing fields: a
// child / splits-from issue link (a split was made) or a scope decision (the
// no-split rationale). No dedicated field is required.
func issueOpsSplitDecisionMissing(record IssueOpsRecord) []string {
	for _, link := range record.IssueLinks {
		switch strings.ToLower(strings.TrimSpace(link.Type)) {
		case "child", "splits-from":
			return nil
		}
	}
	for _, decision := range record.Decisions {
		if strings.EqualFold(strings.TrimSpace(decision.Kind), "scope") {
			return nil
		}
	}
	return []string{"split_decision"}
}

func issueOpsDomainReviewMissing(record IssueOpsRecord) []string {
	if record.DomainReview == nil || strings.TrimSpace(record.DomainReview.ReviewedAt) == "" {
		return []string{"domain_review"}
	}
	return nil
}

func issueOpsPlanCompletion(record IssueOpsRecord) IssueOpsReadiness {
	// Plan completion (branch_prepare + worktree + plan + design_review) is
	// exactly today's "ready to enter compatibility-review" gate.
	return IssueOpsCompatibilityReviewReadiness(record)
}

func issueOpsCompatibilityReviewCompletion(record IssueOpsRecord) IssueOpsReadiness {
	return issueOpsReadinessFrom(record, issueOpsCompatibilityReviewMissing(record))
}

func issueOpsImplementCompletion(record IssueOpsRecord) IssueOpsReadiness {
	// Implement completion adds implementation_changes (an exit artifact) on top
	// of implement-entry readiness — exactly today's ai-slop-clean entry gate.
	return IssueOpsAISlopCleanReadiness(record)
}

func issueOpsAISlopCleanCompletion(record IssueOpsRecord) IssueOpsReadiness {
	missing := []string{}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean_at")
	}
	if strings.TrimSpace(record.AISlopCleanHead) == "" {
		missing = append(missing, "ai_slop_clean_head")
	}
	if strings.TrimSpace(record.AISlopCleanFingerprint) == "" {
		missing = append(missing, "ai_slop_clean_fingerprint")
	}
	if len(cleanIssueOpsTextValues(record.AISlopCleanCategories)) == 0 {
		missing = append(missing, "cleanup_evidence")
	}
	if len(cleanIssueOpsTextValues(record.AISlopCleanVerification)) == 0 {
		missing = append(missing, "verification_evidence")
	}
	return issueOpsReadinessFrom(record, missing)
}

func issueOpsFeedbackCompletion(record IssueOpsRecord) IssueOpsReadiness {
	missing := []string{}
	for _, item := range record.Feedback {
		if strings.TrimSpace(item.Classification) == "" {
			missing = append(missing, "feedback_classification")
			break
		}
	}
	if issueOpsHasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	for _, item := range record.Feedback {
		if strings.TrimSpace(item.Resolution) == "" {
			missing = append(missing, "feedback_resolution")
			break
		}
	}
	return issueOpsReadinessFrom(record, missing)
}

func issueOpsPRCompletion(record IssueOpsRecord) IssueOpsReadiness {
	// Completion/derivation uses the non-strict readiness (no git fetch) so the
	// ledger can be derived for status display without network side effects. The
	// actual pr-phase entry gate still uses IssueOpsStrictPRReadiness.
	ready := IssueOpsPRReadiness(record)
	missing := append([]string{}, ready.Missing...)
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact")
	}
	missing = append(missing, issueOpsTargetBranchMatchMissing(record)...)
	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

// issueOpsTargetBranchMatchMissing reports target_branch_match when a remote
// artifact exists but its target branch does not equal branch_prepare.base_branch.
// When the comparison inputs are not yet captured it is silent (remote_artifact
// covers absence), so it never deadlocks pr entry.
func issueOpsTargetBranchMatchMissing(record IssueOpsRecord) []string {
	if record.RemoteArtifact == nil || record.BranchPrepare == nil {
		return nil
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if base == "" {
		return nil
	}
	if strings.TrimSpace(record.RemoteArtifact.TargetBranch) != base {
		return []string{"target_branch_match"}
	}
	return nil
}

func issueOpsDoneCompletion(record IssueOpsRecord) IssueOpsReadiness {
	missing := []string{}
	if issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhasePR) {
		missing = append(missing, "prior_phase_pr")
	}
	missing = append(missing, issueOpsRemoteArtifactMissing(record)...)
	return issueOpsReadinessFrom(record, missing)
}

// IssueOpsPhaseCompletion computes whether a phase is complete from existing
// source-of-truth fields and returns ready/artifacts(missing). It indexes
// existing readiness functions; it never becomes the source of truth itself.
func IssueOpsPhaseCompletion(record IssueOpsRecord, phase IssueOpsPhase) IssueOpsReadiness {
	switch phase {
	case IssueOpsPhaseProblem:
		return IssueOpsProblemReadiness(record)
	case IssueOpsPhaseGrill:
		return IssueOpsGrillReadiness(record)
	case IssueOpsPhasePlan:
		return issueOpsPlanCompletion(record)
	case IssueOpsPhaseCompatibilityReview:
		return issueOpsCompatibilityReviewCompletion(record)
	case IssueOpsPhaseImplement:
		return issueOpsImplementCompletion(record)
	case IssueOpsPhaseAISlopClean:
		return issueOpsAISlopCleanCompletion(record)
	case IssueOpsPhaseFeedback:
		return issueOpsFeedbackCompletion(record)
	case IssueOpsPhasePR:
		return issueOpsPRCompletion(record)
	case IssueOpsPhaseDone:
		return issueOpsDoneCompletion(record)
	default:
		return IssueOpsReadiness{OK: true, Ready: false, Missing: []string{"unknown_phase"}}
	}
}

// issueOpsPhaseArtifactKeys is the matrix completion set per phase, recorded in
// a completed ledger entry's Artifacts.
func issueOpsPhaseArtifactKeys(phase IssueOpsPhase) []string {
	switch phase {
	case IssueOpsPhaseProblem:
		return []string{"intent_contract"}
	case IssueOpsPhaseGrill:
		return []string{"issue_url", "branch", "plan_prep", "split_decision", "domain_review"}
	case IssueOpsPhasePlan:
		return []string{"branch_prepare", "worktree_path", "plan_path", "design_review"}
	case IssueOpsPhaseCompatibilityReview:
		return []string{"compatibility_review", "compatibility_approval", "compatibility_blockers"}
	case IssueOpsPhaseImplement:
		return []string{"worktree_tools", "execution_decision", "implementation_changes"}
	case IssueOpsPhaseAISlopClean:
		return []string{"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint", "cleanup_evidence", "verification_evidence"}
	case IssueOpsPhaseFeedback:
		return []string{"feedback_classification", "contract_feedback_issue_update", "feedback_resolution"}
	case IssueOpsPhasePR:
		return []string{"strict_pr_readiness", "children_complete", "remote_artifact", "target_branch_match"}
	case IssueOpsPhaseDone:
		return []string{"prior_phase_pr", "verified_remote_artifact"}
	default:
		return nil
	}
}

// DeriveIssueOpsPhaseLedger builds a virtual ledger from current fields, in
// IssueOpsPhases order, using the derived sentinel for timestamps (never
// wall-clock) so output is byte-deterministic. Each entry is marked derived.
// A complete phase records its artifacts and no missing keys; an incomplete
// phase records its missing keys.
func DeriveIssueOpsPhaseLedger(record IssueOpsRecord) IssueOpsPhaseLedger {
	ledger := IssueOpsPhaseLedger{}
	currentRank := issueOpsPhaseRank(record.Phase)
	for _, phase := range model.IssueOpsPhases {
		entry := IssueOpsPhaseLedgerEntry{Phase: phase, Notes: []string{"derived"}}
		if issueOpsPhaseRank(phase) <= currentRank {
			entry.EnteredAt = issueOpsLedgerDerivedSentinel
		}
		completion := IssueOpsPhaseCompletion(record, phase)
		if completion.Ready {
			entry.CompletedAt = issueOpsLedgerDerivedSentinel
			entry.Artifacts = issueOpsPhaseArtifactKeys(phase)
		} else {
			entry.Missing = completion.Missing
		}
		ledger[phase] = entry
	}
	return ledger
}

// IssueOpsStatus reads a cycle for display and ensures the phase ledger is
// present: when no ledger was stamped (e.g. legacy records, or cycles that have
// only recorded artifacts without transitioning), a deterministic derived ledger
// is filled in so status always shows phase progress. It is read-only — the
// derived ledger is not persisted.
func IssueOpsStatus(stateRoot, id string) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if len(record.PhaseLedger) == 0 {
		record.PhaseLedger = DeriveIssueOpsPhaseLedger(record)
	} else {
		// Backfill phases absent from a partial persisted ledger (a multi-phase
		// forward jump only stamps its endpoints; a post-regress ledger persists
		// just the stale plan/compatibility-review entries) so status never
		// under-reports a phase whose artifacts are complete. Read-only: persisted
		// entries (real timestamps, stale notes) always win — only phases missing
		// from the persisted ledger are filled in from the derived ledger.
		derived := DeriveIssueOpsPhaseLedger(record)
		for phase, entry := range derived {
			if _, ok := record.PhaseLedger[phase]; !ok {
				record.PhaseLedger[phase] = entry
			}
		}
	}
	return record, nil
}

// stampIssueOpsForwardTransition records an observed phase transition in the
// ledger (rules 4/5/11): the phase being left is marked complete (a successful
// forward transition means the entry gate for the new phase — which requires the
// previous phase complete — already passed), and the phase being entered gets a
// real entered_at. Timestamps are the genuinely-observed `now`, never the
// derived sentinel. Called only at actual phase-change sites, so it never adds a
// ledger to records that merely recorded an artifact (keeping golden stable).
func stampIssueOpsForwardTransition(ledger IssueOpsPhaseLedger, prevPhase, newPhase IssueOpsPhase, now string) IssueOpsPhaseLedger {
	if ledger == nil {
		ledger = IssueOpsPhaseLedger{}
	}
	prev := ledger[prevPhase]
	prev.Phase = prevPhase
	if prev.EnteredAt == "" {
		prev.EnteredAt = now
	}
	prev.CompletedAt = now
	prev.Artifacts = issueOpsPhaseArtifactKeys(prevPhase)
	prev.Missing = nil
	// A genuinely re-observed forward transition completing this phase clears any
	// stale-regression mark left by RegressIssueOpsForReplan: a phase that has been
	// legitimately re-completed is no longer stale.
	prev.Notes = clearStaleLedgerNotes(prev.Notes)
	ledger[prevPhase] = prev

	entry := ledger[newPhase]
	entry.Phase = newPhase
	if entry.EnteredAt == "" {
		entry.EnteredAt = now
	}
	ledger[newPhase] = entry
	return ledger
}

// clearStaleLedgerNotes removes stale-regression markers (see
// markIssueOpsLedgerStale) from a ledger entry's notes. Called when a forward
// transition re-completes a previously-regressed phase, so the stale mark — which
// no longer applies — does not linger in status forever.
func clearStaleLedgerNotes(notes []string) []string {
	if len(notes) == 0 {
		return notes
	}
	kept := make([]string, 0, len(notes))
	for _, n := range notes {
		if strings.HasPrefix(n, "stale:") {
			continue
		}
		kept = append(kept, n)
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}
