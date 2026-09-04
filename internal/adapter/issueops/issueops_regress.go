package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"issueops/internal/contract/issueops"
)

// RegressIssueOpsForReplan takes the IssueOps feedback loop backward when the
// Brooks devil's advocate (or an equivalent plan review) returns a `stop`
// verdict: it regresses a plan / compatibility-review cycle to grill so the
// scope is re-investigated and the plan redone, rather than blocking in place.
//
// It records the stop reason as a scope decision (audit), clears the rejected
// design's approval so re-entry forces a genuine re-review, and marks the
// downstream plan/compatibility-review ledger entries stale (retained as audit
// per the backward-regression rule). It does not delete the worktree, branch,
// or remote artifacts.
// issueOpsRegressCap bounds stop→reflect→regress rounds per cycle. Each round
// already costs a fresh devil's-advocate verdict plus a remote reflection, so
// repeated rounds signal the plan is thrashing, not converging; past the cap
// the cycle escalates to a human decision instead of another automatic re-plan.
const issueOpsRegressCap = 3

func RegressIssueOpsForReplan(stateRoot, id, reason string) (issueops.IssueOpsRecord, error) {
	return regressIssueOpsForReplan(stateRoot, id, reason, nil)
}

func RegressIssueOpsForReplanWithActor(stateRoot, id, reason string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return regressIssueOpsForReplan(stateRoot, id, reason, &actor)
}

func regressIssueOpsForReplan(stateRoot, id, reason string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("regression reason is required (the Brooks stop verdict)")
	}
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = regressIssueOpsForReplanLocked(stateRoot, id, reason)
		return e
	})
	return rec, err
}

func regressIssueOpsForReplanLocked(stateRoot, id, reason string) (issueops.IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	rank := issueOpsPhaseRank(record.Phase)
	if rank < issueOpsPhaseRank(IssueOpsPhasePlan) || rank > issueOpsPhaseRank(IssueOpsPhaseCompatibilityReview) {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("design-review regression only applies from plan or compatibility-review phase, not %s", record.Phase)
	}
	// A regress is the machine consequence of a devil's-advocate stop whose
	// findings were reflected into the issue, so require both before rewinding.
	review := record.DevilsAdvocateReview
	if review != nil && review.Verdict == "revise" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf(
			"devil's-advocate revise verdict must be resolved in place: update the linked plan, run and record a fresh devil's-advocate review, and proceed only after it passes or is explicitly waived; only a stop verdict may regress after remote reflection")
	}
	if review == nil || review.Verdict != "stop" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("regress requires a recorded devil's-advocate stop verdict")
	}
	if strings.TrimSpace(review.IssueReflectedAt) == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("reflect the devil's-advocate findings to the issue before regressing (issueops remote reflect-devils-advocate --confirm)")
	}
	if len(record.RegressEvents) >= issueOpsRegressCap {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf(
			"regress cap reached: cycle %s already went through %d stop→re-plan rounds, so the plan is thrashing rather than converging; a human decision is required before any further automatic re-plan",
			id, len(record.RegressEvents))
	}
	activeChildren, err := issueOpsActiveChildIDs(stateRoot, record)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	if len(activeChildren) > 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("children_active: %s", strings.Join(activeChildren, ", "))
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	priorPhase := record.Phase

	// Audit trail backing the cap: one event per successful regression.
	record.RegressEvents = append(record.RegressEvents, issueops.IssueOpsRegressEvent{
		Reason:    reason,
		FromPhase: priorPhase,
		At:        now,
	})

	// Audit: record the devil's-advocate stop as a scope decision.
	record.Decisions = append(record.Decisions, issueops.IssueOpsDecision{
		Title:     "design-review devil's-advocate stop",
		Body:      reason,
		Kind:      "scope",
		Rationale: fmt.Sprintf("regressed from %s to grill for re-plan", priorPhase),
		CreatedAt: now,
	})

	// Force a genuine re-plan: the rejected design must be re-reviewed before the
	// cycle can advance past plan again.
	if record.DesignReview != nil {
		record.DesignReview.Approved = false
	}

	// Rule 12: retain the now-ahead plan/compatibility-review ledger entries as
	// audit, but mark them stale so they no longer read as complete. A cycle can
	// reach plan/compatibility-review with an empty or partial ledger (linking and
	// compatibility-review recorders don't stamp it), so this may persist just the
	// two stale entries; IssueOpsStatus backfills the remaining phases for display
	// rather than persisting a derived ledger here (keeping derived ledgers
	// out of the persisted state).
	record.PhaseLedger = markIssueOpsLedgerStale(record.PhaseLedger, reason,
		IssueOpsPhasePlan, IssueOpsPhaseCompatibilityReview)

	// Clear the consumed devil's-advocate review so the re-planned cycle must earn
	// a fresh verdict before implement (the gate re-fires).
	record.DevilsAdvocateReview = nil

	record.Phase = IssueOpsPhaseGrill
	record.UpdatedAt = now
	return touchAndWriteIssueOps(stateRoot, record)
}

// markIssueOpsLedgerStale marks the given phases' ledger entries stale: their
// completion is cleared and a stale note is appended, while the entry is kept
// for audit. It is safe on a nil/empty ledger.
func markIssueOpsLedgerStale(ledger issueops.IssueOpsPhaseLedger, reason string, phases ...issueops.IssueOpsPhase) issueops.IssueOpsPhaseLedger {
	if ledger == nil {
		ledger = issueops.IssueOpsPhaseLedger{}
	}
	note := "stale: design-review regression (" + reason + ")"
	for _, phase := range phases {
		entry := ledger[phase]
		entry.Phase = phase
		entry.CompletedAt = ""
		entry.Notes = append(entry.Notes, note)
		ledger[phase] = entry
	}
	return ledger
}
