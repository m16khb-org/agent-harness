package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

// Direction A: a fixture WITHOUT pioneer_skill_target must score exactly as it
// did before the pioneer dimension existed (true N/A: excluded from
// average/minimum/Passed), while the dimension is still visibly recorded.
func TestPioneerDimensionNAExcludedForNonTargetFixture(t *testing.T) {
	fixture := issueopscontract.IssueOpsBenchmarkFixture{ID: "fixture"}
	artifact := completeBenchmarkArtifactForTest()

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)

	if score.AverageScore != 100 || score.MinimumScore != 100 {
		t.Fatalf("non-target fixture must keep pre-dimension numeric scores, got avg=%v min=%v failures=%v", score.AverageScore, score.MinimumScore, score.DeterministicFailures)
	}
	if !score.Passed {
		t.Fatalf("non-target fixture must still pass: %+v", score)
	}
	found := false
	for _, dim := range score.DimensionScores {
		if dim.Dimension == "pioneer_skill_contribution" {
			found = true
			if !dim.NotApplicable {
				t.Fatalf("pioneer dimension must be NotApplicable for non-target fixture: %+v", dim)
			}
		}
	}
	if !found {
		t.Fatal("pioneer dimension entry must still be recorded as N/A")
	}
}

// Direction B (silent-no-op guard): a pioneer-targeted fixture with EMPTY
// evidence must actually participate in scoring — minimum drops to 0 and a
// pioneer deterministic failure is recorded. Without this, the N/A exclusion
// could silently apply to every fixture and the feature would be a no-op.
func TestPioneerDimensionParticipatesForTargetFixture(t *testing.T) {
	fixture := issueopscontract.IssueOpsBenchmarkFixture{ID: "pioneer-algorithm-optimization", PioneerSkillTarget: "algorithm-optimization"}
	artifact := completeBenchmarkArtifactForTest()
	artifact.PioneerSkillEvidence = ""

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)

	if score.MinimumScore != 0 {
		t.Fatalf("target fixture with empty evidence must drag minimum to 0, got %v", score.MinimumScore)
	}
	if score.Passed {
		t.Fatalf("target fixture with empty evidence must not pass: %+v", score)
	}
	foundFailure := false
	for _, failure := range score.DeterministicFailures {
		if strings.Contains(strings.ToLower(failure), "pioneer") {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Fatalf("expected a pioneer deterministic failure, got %v", score.DeterministicFailures)
	}
}
