package operationalhealth

import (
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestCycleFromRecordProjectsDurableFailures(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID:    "io-failed",
		Phase: issueopscontract.IssueOpsPhaseImplement,
		Execution: &issueopscontract.Execution{
			Failure: &issueopscontract.ExecutionFailure{Code: "external_operation_ambiguous"},
		},
		CleanupFinishFailure: &issueopscontract.IssueOpsCleanupFinishFailure{
			Step: issueopscontract.CleanupFailureStepWorktreeRemove,
		},
	}

	cycle, _ := cycleFromRecord(record, nil)

	if !cycle.ExecutionFailurePresent || !cycle.CleanupFailurePresent {
		t.Fatalf("failure projection = %+v", cycle)
	}
}
