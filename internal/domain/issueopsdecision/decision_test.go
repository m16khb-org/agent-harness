package issueopsdecision

import (
	"strings"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestBuildRejectsSecretAndOversizedFields(t *testing.T) {
	now := time.Date(2026, 8, 11, 6, 0, 0, 0, time.UTC)
	base := issueopscontract.IssueOpsDecisionRecordRequest{
		Kind:  "architecture",
		Title: "Boundary",
		Body:  "Move the decision capability.",
	}
	secret := base
	secret.Body = "token=secret-value"
	if _, err := Build(secret, now); err == nil {
		t.Fatal("secret-like decision body must be rejected")
	}
	oversized := base
	oversized.Title = strings.Repeat("a", 513)
	if _, err := Build(oversized, now); err == nil {
		t.Fatal("oversized title must be rejected")
	}
}
