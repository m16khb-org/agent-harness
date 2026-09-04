package issueops

import (
	"fmt"
	"strings"
	"time"

	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/contract/issueops"
)

func refreshIssueOpsAISlopClean(stateRoot string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	if ready := IssueOpsAISlopCleanReadiness(record); !ready.Ready {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("cannot refresh ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
	}
	record.AISlopCleanAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.AISlopCleanHead = issueOpsCurrentHead(record)
	record.AISlopCleanFingerprint = implementation.ChangeFingerprint(record)
	return touchAndWriteIssueOps(stateRoot, record)
}

func shouldRefreshIssueOpsAISlopClean(record issueops.IssueOpsRecord, phase issueops.IssueOpsPhase) bool {
	if phase != IssueOpsPhaseAISlopClean {
		return false
	}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		return false
	}
	return issueOpsPhaseRank(record.Phase) > issueOpsPhaseRank(IssueOpsPhaseAISlopClean)
}
