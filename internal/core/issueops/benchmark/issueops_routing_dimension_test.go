package benchmark

import (
	"strings"
	"testing"
)

// A2/A5 codd keyword evidence that satisfies issueOpsPioneerSkillEvidenceComplete
// (index AND write-penalty AND a normalization/selectivity term).
const coddKeywordEvidence = "codd method: compared covering index vs partial index on (user_id, created_at); write penalty +8% insert cost; selectivity 0.99 at row count 12M, read:write 40:1."

// Same-entry pairing: the expected (phase,skill) must match ONE trace entry on
// BOTH fields. A trace with the right skill at the WRONG phase must fail.
func TestSkillRoutingFidelitySameEntryPairing(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ExpectedRouting: []SkillRouting{{Phase: "plan", Skill: "codd"}}}

	matched := IssueOpsBenchmarkArtifact{RoutingTrace: []SkillRouting{{Phase: "Plan", Skill: "CODD"}}}
	if !issueOpsSkillRoutingFidelityComplete(fixture, matched) {
		t.Fatal("case-insensitive same-entry pairing must pass")
	}

	// 'plan' present (from hopper entry) and 'codd' present (from review entry),
	// but codd never fired at plan -> must FAIL same-entry pairing.
	crossPaired := IssueOpsBenchmarkArtifact{RoutingTrace: []SkillRouting{{Phase: "plan", Skill: "hopper"}, {Phase: "review", Skill: "codd"}}}
	if issueOpsSkillRoutingFidelityComplete(fixture, crossPaired) {
		t.Fatal("cross-paired trace (right skill at wrong phase) must FAIL same-entry pairing")
	}

	if issueOpsSkillRoutingFidelityComplete(fixture, IssueOpsBenchmarkArtifact{}) {
		t.Fatal("empty trace must fail when routing is expected")
	}
}

// Direction A: a fixture WITHOUT expected_routing keeps pre-dimension scores
// (true N/A: excluded from average/minimum/Passed) while still recorded.
func TestRoutingDimensionNAExcludedForNonRoutingFixture(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "fixture"}
	score := ScoreIssueOpsBenchmarkArtifact(fixture, completeBenchmarkArtifactForTest())

	if score.AverageScore != 100 || score.MinimumScore != 100 || !score.Passed {
		t.Fatalf("non-routing fixture must keep 100/pass: avg=%v min=%v failures=%v", score.AverageScore, score.MinimumScore, score.DeterministicFailures)
	}
	found := false
	for _, dim := range score.DimensionScores {
		if dim.Dimension == "skill_routing_fidelity" {
			found = true
			if !dim.NotApplicable {
				t.Fatalf("routing dimension must be NotApplicable for non-routing fixture: %+v", dim)
			}
		}
	}
	if !found {
		t.Fatal("routing dimension entry must still be recorded as N/A")
	}
}

// Direction B (silent-no-op guard): a routing fixture with an empty trace must
// actually participate — minimum drops to 0 and a routing failure is recorded.
func TestRoutingDimensionParticipatesForRoutingFixture(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "f", ExpectedRouting: []SkillRouting{{Phase: "plan", Skill: "codd"}}}
	score := ScoreIssueOpsBenchmarkArtifact(fixture, completeBenchmarkArtifactForTest())

	if score.MinimumScore != 0 || score.Passed {
		t.Fatalf("routing fixture with empty trace must drag minimum to 0 / not pass: %+v", score)
	}
	foundFailure := false
	for _, failure := range score.DeterministicFailures {
		if strings.Contains(strings.ToLower(failure), "routing") {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Fatalf("expected a routing deterministic failure, got %v", score.DeterministicFailures)
	}
}

// ACCEPTANCE (A5): the routing dimension catches the "keyword present but skill
// did not fire" gap that the keyword proxy alone misses. The fixture carries
// ONLY the routing critical rule (not "skips pioneer method"), isolating the two
// axes so the ONLY thing distinguishing tampered from clean is routing.
func TestSkillRoutingFidelityCatchesKeywordWithoutRouting(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{
		ID:                 "routing-boundary",
		PioneerSkillTarget: "codd",
		ExpectedRouting:    []SkillRouting{{Phase: "plan", Skill: "codd"}},
		CriticalFailures:   []string{"skips expected routing"},
	}

	clean := completeBenchmarkArtifactForTest()
	clean.PioneerSkillEvidence = coddKeywordEvidence
	clean.RoutingTrace = []SkillRouting{{Phase: "plan", Skill: "codd"}}
	cleanScore := ScoreIssueOpsBenchmarkArtifact(fixture, clean)

	if dimScore(cleanScore, "skill_routing_fidelity") != 100 {
		t.Fatalf("clean artifact routing dim must be live and 100, got %v", dimScore(cleanScore, "skill_routing_fidelity"))
	}
	if dimScore(cleanScore, "pioneer_skill_contribution") != 100 || !cleanScore.Passed {
		t.Fatalf("clean artifact must pass both pioneer and routing: %+v", cleanScore)
	}

	// Skill did NOT fire (trace emptied) but the keyword evidence is kept.
	tampered := clean
	tampered.RoutingTrace = nil
	tamperedScore := ScoreIssueOpsBenchmarkArtifact(fixture, tampered)

	if dimScore(tamperedScore, "pioneer_skill_contribution") != 100 {
		t.Fatalf("keyword proxy must STILL pass on tampered artifact, got %v", dimScore(tamperedScore, "pioneer_skill_contribution"))
	}
	if dimScore(tamperedScore, "skill_routing_fidelity") != 0 {
		t.Fatalf("routing dim must catch the missing trace (score 0), got %v", dimScore(tamperedScore, "skill_routing_fidelity"))
	}
	if tamperedScore.Passed {
		t.Fatalf("tampered artifact must NOT pass: %+v", tamperedScore)
	}
	// Single-axis isolation: only the routing failure changed between clean and
	// tampered (clean had none; tampered has exactly the routing one).
	if len(cleanScore.DeterministicFailures) != 0 {
		t.Fatalf("clean artifact must have zero deterministic failures, got %v", cleanScore.DeterministicFailures)
	}
	if len(tamperedScore.DeterministicFailures) != 1 || !strings.Contains(strings.ToLower(tamperedScore.DeterministicFailures[0]), "routing") {
		t.Fatalf("tampered must have exactly one (routing) deterministic failure, got %v", tamperedScore.DeterministicFailures)
	}
	foundCritical := false
	for _, c := range tamperedScore.CriticalFailures {
		if strings.Contains(strings.ToLower(c), "skips expected routing") {
			foundCritical = true
		}
	}
	if !foundCritical {
		t.Fatalf("missing routing must hard-fail via the paired critical rule, got %v", tamperedScore.CriticalFailures)
	}
}

func dimScore(score IssueOpsBenchmarkScore, dimension string) float64 {
	for _, dim := range score.DimensionScores {
		if dim.Dimension == dimension {
			return dim.Score
		}
	}
	return -1
}
