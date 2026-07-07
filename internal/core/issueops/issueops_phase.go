package issueops

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/implementation"
	"agent-harness/internal/core/issueops/model"
)

func knownIssueOpsPhase(phase IssueOpsPhase) bool {
	return model.KnownIssueOpsPhase(phase)
}

func issueOpsPhaseRank(phase IssueOpsPhase) int {
	return model.IssueOpsPhaseRank(phase)
}

func IssueOpsPhaseExpectsWorktree(phase IssueOpsPhase) bool {
	return model.IssueOpsPhaseExpectsWorktree(phase)
}

func AdvanceIssueOpsPhase(stateRoot, id, to string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = advanceIssueOpsPhaseLocked(stateRoot, id, to)
		return e
	})
	if err == nil && rec.Phase == IssueOpsPhaseDone {
		unbindIssueOpsSessionForCycle(rec)
	}
	return rec, err
}

func advanceIssueOpsPhaseLocked(stateRoot, id, to string) (IssueOpsRecord, error) {
	phase := IssueOpsPhase(strings.TrimSpace(to))
	if !knownIssueOpsPhase(phase) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("unknown issueops phase %q", to)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == phase {
		if phase == IssueOpsPhaseAISlopClean {
			return refreshIssueOpsAISlopClean(stateRoot, record)
		}
		return record, nil
	}
	if shouldRefreshIssueOpsAISlopClean(record, phase) {
		return refreshIssueOpsAISlopClean(stateRoot, record)
	}
	if err := validateIssueOpsPhaseTransition(stateRoot, record, phase); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	record = applyIssueOpsPhaseTransition(record, phase)
	return touchAndWriteIssueOps(stateRoot, record)
}

func validateIssueOpsPhaseTransition(stateRoot string, record IssueOpsRecord, phase IssueOpsPhase) error {
	if record.Phase == IssueOpsPhaseDone {
		return fmt.Errorf("cannot leave done phase")
	}
	if issueOpsPhaseRank(phase) < issueOpsPhaseRank(record.Phase) {
		return fmt.Errorf("cannot move issueops phase backward from %s to %s", record.Phase, phase)
	}
	// Fail-closed (rules 1/8): problem and grill have no other readiness gate, so
	// these are the only enforcement of the problem/grill completion contracts.
	// grill entry requires problem complete; plan entry requires grill complete.
	// Downstream phases keep their own readiness gates (which transitively require
	// plan, and thus grill, on the normal sequential path).
	if phase == IssueOpsPhaseGrill {
		if ready := IssueOpsProblemReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter grill phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhasePlan {
		// Plan readiness first: it carries intent_contract/issue_url/plan_prep, so
		// the most fundamental missing key surfaces before the grill-completion
		// delta (split_decision/domain_review/branch).
		if ready := IssueOpsPlanReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter plan phase: missing %s", strings.Join(ready.Missing, ", "))
		}
		if ready := IssueOpsGrillReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter plan phase: grill incomplete: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseCompatibilityReview {
		if ready := IssueOpsCompatibilityReviewReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter compatibility-review phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseImplement {
		if ready := IssueOpsImplementationReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter implement phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseAISlopClean {
		if ready := IssueOpsAISlopCleanReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseFeedback && strings.TrimSpace(record.AISlopCleanAt) == "" {
		return fmt.Errorf("cannot enter feedback phase before ai-slop-clean phase")
	}
	if phase == IssueOpsPhasePR {
		if ready := issueOpsStrictPRReadinessWithState(stateRoot, record); !ready.Ready {
			return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseDone && record.Phase != IssueOpsPhasePR {
		return fmt.Errorf("cannot enter done phase before pr phase")
	}
	if phase == IssueOpsPhaseDone {
		if missing := issueOpsRemoteArtifactMissing(record); len(missing) > 0 {
			return fmt.Errorf("cannot enter done phase before remote artifact verification: missing %s", strings.Join(missing, ", "))
		}
	}
	return nil
}

func applyIssueOpsPhaseTransition(record IssueOpsRecord, phase IssueOpsPhase) IssueOpsRecord {
	prevPhase := record.Phase
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Phase = phase
	if phase == IssueOpsPhaseAISlopClean && strings.TrimSpace(record.AISlopCleanAt) == "" {
		record.AISlopCleanAt = now
	}
	if phase == IssueOpsPhaseAISlopClean {
		record.AISlopCleanHead = issueOpsCurrentHead(record)
		record.AISlopCleanFingerprint = implementation.ChangeFingerprint(record)
	}
	record.PhaseLedger = stampIssueOpsForwardTransition(record.PhaseLedger, prevPhase, phase, now)
	return record
}
