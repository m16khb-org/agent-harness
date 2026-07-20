package issueops

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/implementation"
	"agent-harness/internal/core/issueops/model"
	"context"
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
	return advanceIssueOpsPhaseWithActor(stateRoot, id, to, nil)
}

func AdvanceIssueOpsPhaseWithActor(stateRoot, id, to string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return advanceIssueOpsPhaseWithActor(stateRoot, id, to, &actor)
}

func advanceIssueOpsPhaseWithActor(stateRoot, id, to string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validatePostTransferMutation(record, actor); actorErr != nil {
			return actorErr
		}
		if record.ExecutionWorkspace != nil && (record.ExecutionHandoff == nil || record.ExecutionHandoff.ProtocolVersion != handoff.OwnershipTransferProtocolVersion) {
			if actor == nil {
				return fmt.Errorf("workspace preparation requires a native actor; use the actor-aware phase recorder")
			}
			if actorErr := validateReadyWorkspacePreparationActor(record, *actor); actorErr != nil {
				return actorErr
			}
		}
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
	if err := issueOpsTerminalPhaseHandoffGuard(record, phase); err != nil {
		return err
	}
	return nil
}

// issueOpsTerminalPhaseHandoffGuard rejects advancing a cycle to a terminal
// phase (done) while its supervised execution handoff is still non-terminal
// (state != closed) — the #2581 inconsistency (Task F3). A done-phase record
// with a recovery_required handoff still owns un-reconciled Orca artifacts and
// keeps fencing the source checkout, so write-time prevention closes the source
// of that surprise. The caller is pointed at the exact recover escape; this
// never auto-releases the handoff.
func issueOpsTerminalPhaseHandoffGuard(record IssueOpsRecord, phase IssueOpsPhase) error {
	if phase != IssueOpsPhaseDone {
		return nil
	}
	h := record.ExecutionHandoff
	if h == nil || h.State == handoff.StateClosed || h.ProtocolVersion == handoff.OwnershipTransferProtocolVersion && h.State == handoff.StateCleanupPendingHumanDecision && h.Completion != nil {
		return nil
	}
	return fmt.Errorf("cannot advance to done while the supervised handoff is non-terminal (handoff state=%s); recover it first from the source checkout: agent-harness issueops handoff recover --id %s --action <cancel|finalize-cancel|approve-cleanup> --confirm", h.State, record.ID)
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
