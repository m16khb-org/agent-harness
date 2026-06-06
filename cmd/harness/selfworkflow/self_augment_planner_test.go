package selfworkflow

import (
	"path/filepath"
	"testing"
)

func TestPlanSelfAugmentationUsesGeniusThinkAndScoreGate(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	HarnessRoot = func() string {
		return root
	}
	result := PlanSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	if !result.OK || result.LoopKind != "self_augmentation" || result.KoreanName != selfAugmentationKoreanName {
		t.Fatalf("unexpected loop identity: %+v", result)
	}
	if !result.UsesGeniusThink || len(result.SelectedFormulas) < 2 {
		t.Fatalf("expected GENIUS_THINK formulas: %+v", result.SelectedFormulas)
	}
	if len(result.Candidates) < 10 {
		t.Fatalf("expected candidate curriculum: %+v", result.Candidates)
	}
	if result.SelectedCandidate != nil && result.SelectedCandidate.Status != selfAugmentCandidateStatusOpen {
		t.Fatalf("selected candidate must be an open improvement, got %+v", result.SelectedCandidate)
	}
	if candidateByID(result.Candidates, "loop-taxonomy-score-gates").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("completed taxonomy candidate should be kept for audit but skipped for selection: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "durable-augmentation-memory").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("durable memory candidate should be satisfied after state capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "reflexion-state-memory").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("reflexion memory candidate should be satisfied after lesson capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "adapter-contract-matrix").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("adapter contract matrix candidate should be satisfied after matrix golden support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "qa-race-tier").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("QA race tier candidate should be satisfied after risk-tier QA support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "repo-local-augmentation-sandbox").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("repo-local sandbox candidate should be satisfied after path boundary hardening: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "performance-baseline").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("performance baseline candidate should be satisfied after slow-step compare support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "genius-mermaid-lint").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("Mermaid lint candidate should be satisfied after QA lint support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "install-dry-run-mode").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("install dry-run candidate should be satisfied after dry-run planning support: %+v", result.Candidates)
	}

	for _, id := range []string{"cli-mcp-adapter-split", "dto-compatibility-contract", "candidate-refill-curriculum", "policy-audit-redaction", "worker-mvp-no-shell"} {
		if candidateByID(result.Candidates, id).Status != selfAugmentCandidateStatusSatisfied {
			t.Fatalf("%s should be satisfied after recommended implementation work: %+v", id, result.Candidates)
		}
	}
	if candidateByID(result.Candidates, "release-repro-pack").Status != selfAugmentCandidateStatusOpen {
		t.Fatalf("release reproducibility should remain as the next refill candidate: %+v", result.Candidates)
	}
	if result.SelectedCandidate == nil {
		for _, candidate := range result.Candidates {
			if candidate.Status == selfAugmentCandidateStatusOpen {
				t.Fatalf("nil selected candidate is valid only when no open candidates remain: %+v", result.Candidates)
			}
		}
	}
	if result.TerminationEligible {
		t.Fatalf("planner must not claim implementation termination before a diff is applied")
	}
}

func TestExportSelfVerificationCandidatesSelectsNextOpenCandidate(t *testing.T) {
	result := ExportSelfVerificationCandidates()
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

func candidateByID(candidates []SelfAugmentCandidate, id string) SelfAugmentCandidate {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate
		}
	}
	return SelfAugmentCandidate{}
}
