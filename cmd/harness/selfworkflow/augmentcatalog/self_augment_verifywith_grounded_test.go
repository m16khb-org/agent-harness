package augmentcatalog

import (
	"testing"

	"agent-harness/internal/core/qualitycatalog"
)

// Every self-augment candidate (base + refilled quality specs) must carry an
// explicit verification kind and a VerifyWith that NAMES an external mechanism
// appropriate to that kind — never model self-critique (B1 self-correction
// guardrail). Before B1 only the 10 refilled quality candidates were checked
// for non-empty VerifyWith; the 21 base candidates had no grounding enforcement.
func TestEverySelfAugmentCandidateVerifyWithIsGrounded(t *testing.T) {
	candidates := SelfAugmentCandidates(SelfAugmentRepoSignals{})
	if len(candidates) < 20 {
		t.Fatalf("expected the full candidate catalog, got %d", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.VerificationKind == "" {
			t.Fatalf("candidate %q has no verification kind", candidate.ID)
		}
		if err := qualitycatalog.VerifyWithGrounded(candidate.VerificationKind, candidate.VerifyWith); err != nil {
			t.Fatalf("candidate %q VerifyWith not grounded: %v", candidate.ID, err)
		}
	}
}
