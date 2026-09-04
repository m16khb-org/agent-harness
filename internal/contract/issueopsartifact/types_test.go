package issueopsartifact

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestRecoveryErrorContract(t *testing.T) {
	err := &RecoveryError{ID: "io-42"}

	if err.Error() == "" {
		t.Fatal("RecoveryError message must not be empty")
	}
	fields := err.IssueOpsErrorFields()
	if fields["code"] != "artifact_stage_requires_reseed" {
		t.Fatalf("code = %v, want artifact_stage_requires_reseed", fields["code"])
	}
	if fields["required_action"] != "execution replace --reseed" {
		t.Fatalf("required_action = %v", fields["required_action"])
	}
	next, _ := fields["next_command"].(string)
	if !strings.Contains(next, "io-42") {
		t.Fatalf("next_command %q must reference cycle id", next)
	}
}

func TestAliasesShareUnderlyingIssueOpsContract(t *testing.T) {
	var record issueopscontract.IssueOpsRecord = Record{OK: true, ID: "io-1"}
	if !record.OK || record.ID != "io-1" {
		t.Fatalf("Record alias mismatch: %+v", record)
	}

	staged := Staged{"plan": "# plan"}
	if staged["plan"] != "# plan" {
		t.Fatalf("Staged alias mismatch: %+v", staged)
	}

	if ExecutionModeOrca != issueopscontract.ExecutionModeOrca {
		t.Fatal("ExecutionModeOrca alias drifted from source contract")
	}
	if LeaseStatusReleased != issueopscontract.LeaseStatusReleased {
		t.Fatal("LeaseStatusReleased alias drifted from source contract")
	}
	if PhaseDone != issueopscontract.IssueOpsPhaseDone {
		t.Fatal("PhaseDone alias drifted from source contract")
	}
	if MaxBytes <= 0 {
		t.Fatalf("MaxBytes = %d, want positive limit", MaxBytes)
	}
}
