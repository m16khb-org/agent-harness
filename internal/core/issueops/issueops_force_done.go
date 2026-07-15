package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ForceDoneIssueOps advances a cycle to done from the PR phase, bypassing the
// remote_artifact verification requirement. It still requires the record to be
// in the PR phase. Use when remote artifact verification is unavailable but the
// cycle should be completed.
func ForceDoneIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		rec, e = forceDoneIssueOpsLocked(stateRoot, id)
		return e
	})
	if err == nil {
		unbindIssueOpsSessionForCycle(rec)
	}
	return rec, err
}

func forceDoneIssueOpsLocked(stateRoot, id string) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == IssueOpsPhaseDone {
		return record, nil
	}
	if record.Phase != IssueOpsPhasePR {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot force-done from phase %s; must be in pr phase", record.Phase)
	}
	// force-done bypasses remote-artifact verification, not handoff terminality:
	// a non-terminal supervised handoff must be recovered first (Task F3), never
	// stranded behind a done phase.
	if err := issueOpsTerminalPhaseHandoffGuard(record, IssueOpsPhaseDone); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	activeChildren, err := issueOpsActiveChildIDs(stateRoot, record)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	missing := issueOpsRemoteArtifactMissing(record)
	if len(missing) > 0 {
		// Record the skip reason as a force-release; this is a narrower form
		// of force-release that still respects the PR-phase prerequisite.
		// Stamp ForceReleasedAt too, mirroring ForceReleaseIssueOps, so the
		// bypass is auditable rather than leaving only a reason string.
		record.ForceReleaseReason = "force-done: skipped remote artifact verification (missing " + strings.Join(missing, ", ") + ")"
		record.ForceReleasedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if len(activeChildren) > 0 {
		if strings.TrimSpace(record.ForceReleaseReason) == "" {
			record.ForceReleaseReason = "force-done: parent closed with active children"
			record.ForceReleasedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		record.ForceReleaseReason = issueOpsAppendActiveChildrenAudit(record.ForceReleaseReason, activeChildren)
	}
	record.Phase = IssueOpsPhaseDone
	return touchAndWriteIssueOps(stateRoot, record)
}
