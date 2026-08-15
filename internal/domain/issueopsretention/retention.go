package issueopsretention

import (
	"time"

	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
)

func IsPrunable(record issueopsretentioncontract.Record, cutoff time.Time) bool {
	if record.Phase != issueopsretentioncontract.PhaseDone {
		return false
	}
	if record.Execution != nil && record.Execution.Lease.Status != issueopsretentioncontract.LeaseStatusReleased {
		return false
	}
	if record.IssueCreateIntent != nil &&
		record.IssueCreateIntent.Status != issueopsretentioncontract.IssueCreateCompleted {
		return false
	}
	if record.RemoteArtifact != nil &&
		(record.RemoteCompletion == nil || record.RemoteCompletion.ReflectedAt == "") {
		return false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	return err == nil && !updatedAt.IsZero() && updatedAt.Before(cutoff)
}
