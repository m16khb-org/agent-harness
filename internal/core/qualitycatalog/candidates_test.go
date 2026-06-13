package qualitycatalog

import "testing"

func TestCandidatesProjectSpecsIntoOpenCandidates(t *testing.T) {
	specs := CandidateSpecs()
	candidates := Candidates()
	if len(specs) < 10 {
		t.Fatalf("expected at least 10 quality specs, got %d", len(specs))
	}
	if len(candidates) != len(specs) {
		t.Fatalf("got %d candidates, want %d", len(candidates), len(specs))
	}
	for i, candidate := range candidates {
		spec := specs[i]
		if candidate.ID != spec.ID || candidate.Title != spec.Title || candidate.Category != spec.Category {
			t.Fatalf("candidate %d did not preserve identity: %#v from %#v", i, candidate, spec)
		}
		if candidate.Status != CandidateStatusOpen {
			t.Fatalf("candidate %s status = %q", candidate.ID, candidate.Status)
		}
		if candidate.Score <= 0 || candidate.Score > 100 {
			t.Fatalf("candidate %s score out of range: %f", candidate.ID, candidate.Score)
		}
		if len(candidate.VerifyWith) == 0 || len(candidate.Evidence) == 0 {
			t.Fatalf("candidate %s missing verification evidence", candidate.ID)
		}
		candidate.VerifyWith[0] = "mutated"
		if specs[i].VerifyWith[0] == "mutated" {
			t.Fatal("candidate VerifyWith should be a defensive copy")
		}
	}
}

func TestScoreClampsToRange(t *testing.T) {
	if got := Score(200, 200, 200, -200); got != 100 {
		t.Fatalf("high score should clamp to 100, got %f", got)
	}
	if got := Score(-200, -200, -200, 500); got != 0 {
		t.Fatalf("low score should clamp to 0, got %f", got)
	}
	if got := Score(80, 70, 60, 20); got <= 0 || got >= 100 {
		t.Fatalf("ordinary score should remain in range, got %f", got)
	}
}
