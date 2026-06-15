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

// Every quality spec must carry an explicit verification kind and a VerifyWith
// that NAMES an external mechanism for that kind (catalog hygiene — B1).
func TestEveryQualitySpecVerifyWithIsGrounded(t *testing.T) {
	for _, spec := range CandidateSpecs() {
		if spec.VerificationKind == "" {
			t.Fatalf("spec %q has no verification kind", spec.ID)
		}
		if err := VerifyWithGrounded(spec.VerificationKind, spec.VerifyWith); err != nil {
			t.Fatalf("spec %q VerifyWith not grounded: %v", spec.ID, err)
		}
	}
}

// Teeth: a VerifyWith that is ONLY model self-critique must be REJECTED for a
// tool_signal candidate — including phrasings that carry no denylist keyword.
// If any of these pass, the guard is vacuous and this test goes red.
func TestVerifyWithGroundedRejectsSelfCritique(t *testing.T) {
	selfCritiqueOnly := [][]string{
		{"verified by inspection"},
		{"checked it by hand"},
		{"I judged the diff correct"},
		{"manually validated by reading"},
		{"agent reviewed the output and it seemed correct"},
		{"the reviewer confirms it reads well"},
		{"looks correct to me"},
		{"the model assessed the change as reasonable"},
	}
	for _, vw := range selfCritiqueOnly {
		if err := VerifyWithGrounded(ToolSignalKind, vw); err == nil {
			t.Fatalf("self-critique-only VerifyWith must be rejected for tool_signal: %v", vw)
		}
		if err := VerifyWithGrounded(DocArtifactKind, vw); err == nil {
			t.Fatalf("self-critique-only VerifyWith must be rejected for doc_artifact: %v", vw)
		}
	}

	for _, vw := range [][]string{
		{"go test ./internal/core/state -count=1"},
		{"response_contract golden"},
		{"markdown fixture lint"},
		{"temp HOME install smoke"},
		{"harness self-verify --full --iterations=10 --target-score=95"},
	} {
		if err := VerifyWithGrounded(ToolSignalKind, vw); err != nil {
			t.Fatalf("real tool signal must pass: %v -> %v", vw, err)
		}
	}

	// Concrete documentary deliverables pass for doc_artifact but NOT for
	// tool_signal (a doc artifact is not an executable signal).
	for _, vw := range [][]string{
		{"ADR decision entry", "rollback criteria"},
		{"README install/update/rollback section"},
		{"dogfooding notes document", "Codex inspect/docs/state transcript"},
	} {
		if err := VerifyWithGrounded(DocArtifactKind, vw); err != nil {
			t.Fatalf("doc artifact must pass for doc_artifact: %v -> %v", vw, err)
		}
	}

	if err := VerifyWithGrounded(ToolSignalKind, nil); err == nil {
		t.Fatal("empty VerifyWith must be rejected")
	}
	if err := VerifyWithGrounded(ToolSignalKind, []string{"  "}); err == nil {
		t.Fatal("blank entry must be rejected")
	}
	if err := VerifyWithGrounded(VerificationKind("nonsense"), []string{"go test ./..."}); err == nil {
		t.Fatal("unknown verification kind must be rejected")
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
