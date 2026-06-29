package issueops

import (
	"fmt"
	"strings"
	"time"
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
func RegressIssueOpsForReplan(stateRoot, id, reason string) (IssueOpsRecord, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("regression reason is required (the Brooks stop verdict)")
	}
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = regressIssueOpsForReplanLocked(stateRoot, id, reason)
		return e
	})
	return rec, err
}

func regressIssueOpsForReplanLocked(stateRoot, id, reason string) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	rank := issueOpsPhaseRank(record.Phase)
	if rank < issueOpsPhaseRank(IssueOpsPhasePlan) || rank > issueOpsPhaseRank(IssueOpsPhaseCompatibilityReview) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("brooks regression only applies from plan or compatibility-review phase, not %s", record.Phase)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	priorPhase := record.Phase

	// Audit: record the devil's-advocate stop as a scope decision.
	record.Decisions = append(record.Decisions, IssueOpsDecision{
		Title:     "brooks devil's-advocate stop",
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
	// audit, but mark them stale so they no longer read as complete.
	record.PhaseLedger = markIssueOpsLedgerStale(record.PhaseLedger, reason,
		IssueOpsPhasePlan, IssueOpsPhaseCompatibilityReview)

	record.Phase = IssueOpsPhaseGrill
	record.UpdatedAt = now
	return touchAndWriteIssueOps(stateRoot, record)
}

// markIssueOpsLedgerStale marks the given phases' ledger entries stale: their
// completion is cleared and a stale note is appended, while the entry is kept
// for audit. It is safe on a nil/empty ledger.
func markIssueOpsLedgerStale(ledger IssueOpsPhaseLedger, reason string, phases ...IssueOpsPhase) IssueOpsPhaseLedger {
	if ledger == nil {
		ledger = IssueOpsPhaseLedger{}
	}
	note := "stale: brooks regression (" + reason + ")"
	for _, phase := range phases {
		entry := ledger[phase]
		entry.Phase = phase
		entry.CompletedAt = ""
		entry.Notes = append(entry.Notes, note)
		ledger[phase] = entry
	}
	return ledger
}
