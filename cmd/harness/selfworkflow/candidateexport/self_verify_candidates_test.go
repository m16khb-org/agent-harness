package candidateexport

import (
	"path/filepath"
	"testing"
)

func TestExportSelfVerificationCandidatesSelectsNextOpenCandidate(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	result := ExportSelfVerificationCandidates(root)
	if !result.OK || result.Kind != SelfVerificationCandidateExportKind || result.LoopKind != "self_verification" {
		t.Fatalf("unexpected candidate export identity: %+v", result)
	}
	if result.CandidateCount < 10 || len(result.Candidates) != result.CandidateCount {
		t.Fatalf("expected self-verification candidate curriculum: %+v", result)
	}
	if result.SelectedCandidate != nil {
		t.Fatalf("expected no selected candidate after completion-evidence-audit is satisfied, got %s", result.SelectedCandidate.ID)
	}
	if len(result.OpenCandidateIDs) != 0 {
		t.Fatalf("expected no open candidates, got %v", result.OpenCandidateIDs)
	}
	if !containsString(result.SatisfiedCandidateIDs, "completion-evidence-audit") {
		t.Fatalf("expected completion-evidence-audit to be satisfied, got %v", result.SatisfiedCandidateIDs)
	}
	if containsString(result.OpenCandidateIDs, "self-verify-candidate-export") || !containsString(result.SatisfiedCandidateIDs, "self-verify-candidate-export") || containsString(result.OpenCandidateIDs, "self-verify-step-budget-baseline") || !containsString(result.SatisfiedCandidateIDs, "self-verify-step-budget-baseline") || containsString(result.OpenCandidateIDs, "self-verify-install-dry-run-smoke") || !containsString(result.SatisfiedCandidateIDs, "self-verify-install-dry-run-smoke") {
		t.Fatalf("implemented candidates should be satisfied after their evidence exists: open=%v satisfied=%v", result.OpenCandidateIDs, result.SatisfiedCandidateIDs)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
