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
	if candidateByID(result.Candidates, "release-repro-pack").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("release reproducibility should be satisfied after the release reproducibility pack is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "release-user-readme").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("release user README should be satisfied after README install/update/rollback guide is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "cross-platform-build-matrix").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("cross-platform build matrix should be satisfied after release build matrix support is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "distribution-decision-record").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("distribution decision record should be satisfied after ADR and rollback criteria are implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "release-dogfood-notes").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("release dogfood notes should be satisfied after Codex/Claude dogfood transcripts are implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "quality-signal-harvester").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("quality signal harvester should be satisfied after quality inspect signal output is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "self-augment-signal-table").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("self-augment signal table should be satisfied after repo signal collection is table-driven: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "coverage-mcp-resources").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("MCP resource coverage should be satisfied after catalog/read edge coverage is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "coverage-host-judgement").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("external LLM coverage should be satisfied after malformed output, timeout, and command failure coverage is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "coverage-issueops-linking").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("issueops linking coverage should be satisfied after boundary coverage is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "state-write-locking").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("state write locking should be satisfied after StateWrite uses per-key locks: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "coverage-commandguard").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("commandguard coverage should be satisfied after boundary coverage is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "worker-stuck-running-detection").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("worker stuck-running detection should be satisfied after cleanup-stuck support is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "daemon-connection-limit").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("daemon connection limit should be satisfied after accept-loop max connection guard is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "draftwiki-stale-lock").Status != augmentcatalog.SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("draft-wiki stale lock should be satisfied after queue lock stale recovery is implemented: %+v", result.Candidates)
	}
	if result.SelectedCandidate != nil {
		t.Fatalf("expected no selected candidate after all catalog candidates are satisfied, got %+v", result.SelectedCandidate)
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
