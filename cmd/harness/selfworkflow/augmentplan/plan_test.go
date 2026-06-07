package augmentplan

import (
	"path/filepath"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
)

func TestPlanSelfAugmentationUsesGeniusThinkAndScoreGate(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	result := Plan(model.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}, root, "test")
	if !result.OK || result.LoopKind != "self_augmentation" || result.KoreanName != model.SelfAugmentationKoreanName {
		t.Fatalf("unexpected loop identity: %+v", result)
	}
	if !result.UsesGeniusThink || len(result.SelectedFormulas) < 2 {
		t.Fatalf("expected GENIUS_THINK formulas: %+v", result.SelectedFormulas)
	}
	if len(result.Candidates) < 10 {
		t.Fatalf("expected candidate curriculum: %+v", result.Candidates)
	}
	if result.SelectedCandidate != nil && result.SelectedCandidate.Status != augmentcatalog.SelfAugmentCandidateStatusOpen {
		t.Fatalf("selected candidate must be an open improvement, got %+v", result.SelectedCandidate)
	}
	if candidateByID(result.Candidates, "loop-taxonomy-score-gates").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("completed taxonomy candidate should be kept for audit but skipped for selection: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "durable-augmentation-memory").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("durable memory candidate should be satisfied after state capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "reflexion-state-memory").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("reflexion memory candidate should be satisfied after lesson capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "adapter-contract-matrix").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("adapter contract matrix candidate should be satisfied after matrix golden support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "qa-race-tier").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("QA race tier candidate should be satisfied after risk-tier QA support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "repo-local-augmentation-sandbox").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("repo-local sandbox candidate should be satisfied after path boundary hardening: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "performance-baseline").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("performance baseline candidate should be satisfied after slow-step compare support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "genius-mermaid-lint").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("Mermaid lint candidate should be satisfied after QA lint support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "install-dry-run-mode").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("install dry-run candidate should be satisfied after dry-run planning support: %+v", result.Candidates)
	}

	for _, id := range []string{"cli-mcp-adapter-split", "dto-compatibility-contract", "candidate-refill-curriculum", "policy-audit-redaction", "worker-mvp-no-shell"} {
		if candidateByID(result.Candidates, id).Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
			t.Fatalf("%s should be satisfied after recommended implementation work: %+v", id, result.Candidates)
		}
	}
	if candidateByID(result.Candidates, "release-repro-pack").Status != augmentcatalog.SelfAugmentCandidateStatusOpen {
		t.Fatalf("release reproducibility should remain as the next refill candidate: %+v", result.Candidates)
	}
	if result.SelectedCandidate == nil {
		for _, candidate := range result.Candidates {
			if candidate.Status == augmentcatalog.SelfAugmentCandidateStatusOpen {
				t.Fatalf("nil selected candidate is valid only when no open candidates remain: %+v", result.Candidates)
			}
		}
	}
	if result.TerminationEligible {
		t.Fatalf("planner must not claim implementation termination before a diff is applied")
	}
}

func candidateByID(candidates []model.SelfAugmentCandidate, id string) model.SelfAugmentCandidate {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate
		}
	}
	return model.SelfAugmentCandidate{}
}
