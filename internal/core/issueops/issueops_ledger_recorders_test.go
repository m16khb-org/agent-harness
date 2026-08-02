package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func newLedgerRecorderRecord(t *testing.T) (string, string) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	rec, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-ledger"})
	if err != nil {
		t.Fatalf("start issueops: %v", err)
	}
	return stateRoot, rec.ID
}

func TestRecordIssueOpsDomainReview(t *testing.T) {
	stateRoot, id := newLedgerRecorderRecord(t)

	if _, err := RecordIssueOpsDomainReview(stateRoot, id, issueops.IssueOpsDomainReviewRequest{}); err == nil {
		t.Fatal("empty domain review request should be rejected")
	}

	rec, err := RecordIssueOpsDomainReview(stateRoot, id, issueops.IssueOpsDomainReviewRequest{
		Terminology:       []string{"ledger"},
		ModelFit:          "fits the phase model",
		Risks:             []string{"deadlock"},
		OpenUncertainties: []string{"none"},
	})
	if err != nil {
		t.Fatalf("record domain review: %v", err)
	}
	if rec.DomainReview == nil || rec.DomainReview.ModelFit != "fits the phase model" {
		t.Fatalf("domain review not persisted: %#v", rec.DomainReview)
	}
	if strings.TrimSpace(rec.DomainReview.ReviewedAt) == "" {
		t.Fatal("domain review must stamp reviewed_at")
	}
	if r := IssueOpsGrillReadiness(rec); issueOpsDomainReviewMissingForTest(r) {
		t.Fatalf("recorded domain review should satisfy grill domain_review, missing=%#v", r.Missing)
	}
}

func issueOpsDomainReviewMissingForTest(r issueops.IssueOpsReadiness) bool {
	for _, m := range r.Missing {
		if m == "domain_review" {
			return true
		}
	}
	return false
}

func TestRecordIssueOpsAISlopCleanEvidence(t *testing.T) {
	stateRoot, id := newLedgerRecorderRecord(t)

	if _, err := RecordIssueOpsAISlopCleanEvidence(stateRoot, id, nil, []string{"go test"}); err == nil {
		t.Fatal("missing cleanup categories should be rejected")
	}
	if _, err := RecordIssueOpsAISlopCleanEvidence(stateRoot, id, []string{"dead-code"}, nil); err == nil {
		t.Fatal("missing verification should be rejected")
	}

	rec, err := RecordIssueOpsAISlopCleanEvidence(stateRoot, id, []string{"dead-code", "  ", "duplication"}, []string{"go test ./..."})
	if err != nil {
		t.Fatalf("record evidence: %v", err)
	}
	if len(rec.AISlopCleanCategories) != 2 {
		t.Fatalf("blank categories should be cleaned, got %#v", rec.AISlopCleanCategories)
	}
	if len(rec.AISlopCleanVerification) != 1 || rec.AISlopCleanVerification[0] != "go test ./..." {
		t.Fatalf("verification not persisted: %#v", rec.AISlopCleanVerification)
	}
}

func TestResolveIssueOpsFeedback(t *testing.T) {
	stateRoot, id := newLedgerRecorderRecord(t)
	if _, err := AddIssueOpsFeedback(stateRoot, id, "review", "fix the bug", "defect"); err != nil {
		t.Fatalf("add feedback: %v", err)
	}

	if _, err := ResolveIssueOpsFeedback(stateRoot, id, 0, "not-a-resolution"); err == nil {
		t.Fatal("unknown resolution should be rejected")
	}
	if _, err := ResolveIssueOpsFeedback(stateRoot, id, 5, "valid-defect"); err == nil {
		t.Fatal("out-of-range index should be rejected")
	}

	rec, err := ResolveIssueOpsFeedback(stateRoot, id, 0, "valid-defect")
	if err != nil {
		t.Fatalf("resolve feedback: %v", err)
	}
	if len(rec.Feedback) != 1 || rec.Feedback[0].Resolution != "valid-defect" {
		t.Fatalf("resolution not persisted: %#v", rec.Feedback)
	}
}
