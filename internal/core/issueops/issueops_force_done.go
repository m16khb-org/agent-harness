package issueops

import (
	"fmt"
	"strings"
)

// ForceDoneIssueOps advances a cycle to done from the PR phase, bypassing the
// remote_artifact verification requirement. It still requires the record to be
// in the PR phase. Use when remote artifact verification is unavailable but the
// cycle should be completed.
func ForceDoneIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
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
		record.ForceReleaseReason = "force-done: skipped remote artifact verification (missing " + strings.Join(missing, ", ") + ")"
	}
	record.Phase = IssueOpsPhaseDone
	return touchAndWriteIssueOps(stateRoot, record)
}
