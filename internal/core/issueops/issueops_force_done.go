package issueops

import (
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
	err := withIssueOpsLock(stateRoot, id, func() error {
		var e error
		rec, e = forceDoneIssueOpsLocked(stateRoot, id)
		return e
	})
	if err == nil {
		unbindIssueOpsSessionForCycle(rec.Repo, id)
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
	missing := issueOpsRemoteArtifactMissing(record)
	if len(missing) > 0 {
		// Record the skip reason as a force-release; this is a narrower form
		// of force-release that still respects the PR-phase prerequisite.
		// Stamp ForceReleasedAt too, mirroring ForceReleaseIssueOps, so the
		// bypass is auditable rather than leaving only a reason string.
		record.ForceReleaseReason = "force-done: skipped remote artifact verification (missing " + strings.Join(missing, ", ") + ")"
		record.ForceReleasedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	record.Phase = IssueOpsPhaseDone
	return touchAndWriteIssueOps(stateRoot, record)
}
