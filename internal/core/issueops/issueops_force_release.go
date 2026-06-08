package issueops

import (
	"fmt"
	"strings"
	"time"
)

func ForceReleaseIssueOps(stateRoot, id, reason string) (IssueOpsRecord, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("force-release requires a reason")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == IssueOpsPhaseDone {
		return record, nil
	}
	record.Phase = IssueOpsPhaseDone
	record.ForceReleasedAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.ForceReleaseReason = reason
	return touchAndWriteIssueOps(stateRoot, record)
}
