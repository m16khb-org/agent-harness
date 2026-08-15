package issueopsartifact

import (
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestCanStageOnlyRecoveryPlanAfterExecutionPrepare(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		Execution: &issueopscontract.Execution{
			Mode: issueopscontract.ExecutionModeOrca,
			Lease: issueopscontract.WriteLease{
				Generation: 1,
				Status:     issueopscontract.LeaseStatusReleased,
			},
		},
	}
	if !CanStage(record, "plan") {
		t.Fatal("clean released Orca generation must accept a recovery plan")
	}
	if CanStage(record, "spec") {
		t.Fatal("prepared execution must reject non-plan artifacts")
	}
}

func TestCanStageRejectsTerminalRecordWithoutExecution(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{Phase: issueopscontract.IssueOpsPhaseDone}
	if CanStage(record, "plan") {
		t.Fatal("terminal record must reject new staged artifacts")
	}
}

func TestValidateContentRejectsSecretLikeValue(t *testing.T) {
	if err := ValidateContent([]byte("token=secret-value")); err == nil {
		t.Fatal("secret-like artifact must be rejected")
	}
}
