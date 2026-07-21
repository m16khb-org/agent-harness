package stalescan

import (
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

func TestOwnershipCleanupPendingIsNeverAStaleRelease(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-owner-done", Branch: "1-owner", Phase: model.IssueOpsPhaseDone, WorktreePath: "/tmp/owner", ExecutionHandoff: &model.IssueOpsExecutionHandoff{State: handoff.StateCleanupPendingHumanDecision}}
	finding, found := Classify(record, Probe{}, 0)
	if !found || finding.Category != CategoryHumanCleanupPending || finding.Releasable {
		t.Fatalf("ownership completion classification = %#v found=%v", finding, found)
	}
}
