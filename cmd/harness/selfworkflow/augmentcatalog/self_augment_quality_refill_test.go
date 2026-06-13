package augmentcatalog

import "testing"

func TestQualityRefillCandidatesStayOpenWithScoresAndVerification(t *testing.T) {
	wantIDs := []string{
		"quality-signal-harvester",
		"self-augment-signal-table",
		"coverage-commandguard",
		"coverage-mcp-resources",
		"coverage-externalllm",
		"coverage-issueops-linking",
		"daemon-connection-limit",
		"worker-stuck-running-detection",
		"state-write-locking",
		"draftwiki-stale-lock",
	}

	byID := map[string]SelfAugmentCandidate{}
	for _, candidate := range SelfAugmentCandidates(SelfAugmentRepoSignals{}) {
		byID[candidate.ID] = candidate
	}

	for _, id := range wantIDs {
		candidate, ok := byID[id]
		if !ok {
			t.Fatalf("quality refill candidate %q missing", id)
		}
		if candidate.Status != SelfAugmentCandidateStatusOpen {
			t.Fatalf("candidate %q status=%q, want open", id, candidate.Status)
		}
		if candidate.Score <= 0 || candidate.Impact <= 0 || candidate.Feasibility <= 0 || candidate.Risk <= 0 {
			t.Fatalf("candidate %q has incomplete scoring fields: %+v", id, candidate)
		}
		if len(candidate.VerifyWith) == 0 {
			t.Fatalf("candidate %q has no verification commands", id)
		}
	}
}
