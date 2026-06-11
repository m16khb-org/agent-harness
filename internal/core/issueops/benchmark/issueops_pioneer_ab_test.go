package benchmark

import (
	"path/filepath"
	"strings"
	"testing"
)

// These A/B tests verify that the DETECTOR discriminates signature-present vs
// signature-absent evidence through the run/compare/gate wiring. They do NOT
// measure live skill routing: both artifacts are hand-authored, so this is a
// regression gate for the scoring path, not proof that issueops invoked the
// skills (the plan's honesty rule: unmeasured != failing).

func pioneerABFixturesForTest(t *testing.T) []IssueOpsBenchmarkFixture {
	t.Helper()
	fixtures, err := LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "..", "..", "testdata", "issueops", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	var pioneers []IssueOpsBenchmarkFixture
	for _, fixture := range fixtures {
		if strings.HasPrefix(fixture.ID, "pioneer-") {
			pioneers = append(pioneers, fixture)
		}
	}
	if len(pioneers) != 4 {
		t.Fatalf("expected 4 pioneer fixtures, got %d", len(pioneers))
	}
	return pioneers
}

func pioneerABEvidenceForTest(target string) string {
	switch target {
	case "dijkstra":
		return "complexity O(n^2) -> O(n log n); scaling test N=100->10000; before 4.1s after 0.2s"
	case "codd":
		return "covering index vs partial index compared; write penalty +8% insert cost; selectivity 0.99, row count 12M"
	case "hopper":
		return "reproduced the failure; root cause isolated via hypothesis; fix verified by regression test"
	case "shannon":
		return "SNR before 0.62 baseline -> after 0.81; entropy and redundancy re-measured"
	default:
		return ""
	}
}

func pioneerABRunForTest(t *testing.T, fixtures []IssueOpsBenchmarkFixture, withEvidence bool) IssueOpsBenchmarkRunResult {
	t.Helper()
	artifacts := make(map[string]IssueOpsBenchmarkArtifact, len(fixtures))
	for _, fixture := range fixtures {
		artifact := completeBenchmarkArtifactForTest()
		if withEvidence {
			artifact.PioneerSkillEvidence = pioneerABEvidenceForTest(fixture.PioneerSkillTarget)
		} else {
			artifact.PioneerSkillEvidence = ""
		}
		artifacts[fixture.ID] = artifact
	}
	run, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{Fixtures: fixtures, Artifacts: artifacts})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestPioneerSignaturePresentVsAbsentAB(t *testing.T) {
	fixtures := pioneerABFixturesForTest(t)
	absent := pioneerABRunForTest(t, fixtures, false)
	present := pioneerABRunForTest(t, fixtures, true)

	compare := CompareIssueOpsBenchmarkRuns(absent, present)
	if !compare.Improved || compare.AverageScoreDelta <= 0 {
		t.Fatalf("signature-present run must improve over absent: %+v", compare)
	}
	if len(compare.Regressions) != 0 {
		t.Fatalf("unexpected regressions: %v", compare.Regressions)
	}
	if present.CriticalFailureCount != 0 {
		t.Fatalf("present run must clear criticals, got %d", present.CriticalFailureCount)
	}
	if absent.CriticalFailureCount != 4 {
		t.Fatalf("absent run must trip the pioneer critical on all 4 fixtures, got %d", absent.CriticalFailureCount)
	}
}

// When the candidate run has no pioneer fixtures at all, the dimension is
// absent from its minimums map; the comparator must treat that as
// not-comparable rather than reading the map's 0.0 zero value and reporting a
// phantom regression (reviewer-reproduced latent gap).
func TestPioneerDimensionAbsenceIsNotARegression(t *testing.T) {
	pioneers := pioneerABFixturesForTest(t)
	baseline := pioneerABRunForTest(t, pioneers, true)

	workflowOnly := IssueOpsBenchmarkFixture{ID: "workflow-only"}
	candidate, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{
		Fixtures:  []IssueOpsBenchmarkFixture{workflowOnly},
		Artifacts: map[string]IssueOpsBenchmarkArtifact{"workflow-only": completeBenchmarkArtifactForTest()},
	})
	if err != nil {
		t.Fatal(err)
	}

	compare := CompareIssueOpsBenchmarkRuns(baseline, candidate)
	for _, dimension := range compare.Regressions {
		if dimension == "pioneer_skill_contribution" {
			t.Fatalf("absent pioneer dimension must not be a phantom regression: %+v", compare)
		}
	}
}

func TestPioneerGateRejectsSignatureRegression(t *testing.T) {
	fixtures := pioneerABFixturesForTest(t)
	baseline := pioneerABRunForTest(t, fixtures, true)

	// Candidate drops one fixture's signature: the gate must catch the
	// pioneer dimension regression and discard the candidate.
	artifacts := make(map[string]IssueOpsBenchmarkArtifact, len(fixtures))
	for i, fixture := range fixtures {
		artifact := completeBenchmarkArtifactForTest()
		artifact.PioneerSkillEvidence = pioneerABEvidenceForTest(fixture.PioneerSkillTarget)
		if i == 0 {
			artifact.PioneerSkillEvidence = ""
		}
		artifacts[fixture.ID] = artifact
	}
	candidateRun, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{Fixtures: fixtures, Artifacts: artifacts})
	if err != nil {
		t.Fatal(err)
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate: IssueOpsAutoresearchCandidate{
			ID:               "pioneer-ab-gate",
			Hypothesis:       "dropping a pioneer signature must be discarded",
			TargetDimensions: []string{"pioneer_skill_contribution"},
			EditSurface:      []string{"skills/**", "internal/core/issueops/**"},
		},
		BaselineRun:  baseline,
		CandidateRun: candidateRun,
		ChangedPaths: []string{"internal/core/issueops/benchmark/issueops_pioneer_checks.go"},
	})
	if result.KeepCandidate {
		t.Fatalf("gate must discard a candidate that regresses the pioneer dimension: %+v", result)
	}
	foundRegression := false
	for _, dimension := range result.TargetDimensionRegressions {
		if dimension == "pioneer_skill_contribution" {
			foundRegression = true
		}
	}
	if !foundRegression {
		t.Fatalf("expected pioneer_skill_contribution in target regressions: %+v", result)
	}
}
