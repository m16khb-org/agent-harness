package issueopsartifact

import (
	"bytes"
	"strings"

	issueopsartifactcontract "agent-harness/internal/contract/issueopsartifact"

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

func TestNormalizeNameAcceptsOnlyCatalogNames(t *testing.T) {
	for _, name := range []string{"plan", "spec", "turing-loop", "  plan  "} {
		got, err := NormalizeName(name)
		if err != nil || got != strings.TrimSpace(name) {
			t.Fatalf("NormalizeName(%q) = %q, %v", name, got, err)
		}
	}
	for _, bad := range []string{"", "diagram", "PLAN", "turing"} {
		if _, err := NormalizeName(bad); err == nil || !strings.Contains(err.Error(), "plan|spec|turing-loop") {
			t.Fatalf("NormalizeName(%q) must fail with the catalog message: %v", bad, err)
		}
	}
}

func TestValidateContentSizeBoundaries(t *testing.T) {
	if err := ValidateContent(nil); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty content must fail: %v", err)
	}
	if err := ValidateContent([]byte("x")); err != nil {
		t.Fatalf("small content must pass: %v", err)
	}
	oversized := bytes.Repeat([]byte("x"), issueopsartifactcontract.MaxBytes+1)
	if err := ValidateContent(oversized); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized content must fail: %v", err)
	}
	// 상한은 배타적이지 않다: 정확히 MaxBytes는 통과해야 한다.
	if err := ValidateContent(bytes.Repeat([]byte("x"), issueopsartifactcontract.MaxBytes)); err != nil {
		t.Fatalf("exact-limit content must pass: %v", err)
	}
}
