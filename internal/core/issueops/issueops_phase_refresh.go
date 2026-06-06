package issueops

import (
	"fmt"
	"strings"
	"time"
)

func refreshIssueOpsAISlopClean(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	if ready := IssueOpsAISlopCleanReadiness(record); !ready.Ready {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot refresh ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
	}
	record.AISlopCleanAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.AISlopCleanHead = issueOpsCurrentHead(record)
	record.AISlopCleanFingerprint = issueOpsChangeFingerprint(record)
	return touchAndWriteIssueOps(stateRoot, record)
}

func shouldRefreshIssueOpsAISlopClean(record IssueOpsRecord, phase IssueOpsPhase) bool {
	if phase != IssueOpsPhaseAISlopClean {
		return false
	}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		return false
	}
	return issueOpsPhaseRank(record.Phase) > issueOpsPhaseRank(IssueOpsPhaseAISlopClean)
}
