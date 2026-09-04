package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	"reflect"
	"testing"
)

// A3 acceptance (b): the DETERMINISTIC scorer at a fixed input must collapse to
// a point across repeats. The check asserts on the ORDERED, persisted outputs
// (DimensionScores / DeterministicFailures / CriticalFailures) — the fields a
// map-iteration-order regression would scramble — NOT only the commutative
// AverageScore, whose width is a tautological 0 for any pure function and is
// blind to failure-ordering nondeterminism.

// scorerOutputsEqual compares the order-sensitive persisted outputs of the
// scorer (what CompareIssueOpsBenchmarkRuns and JSON persistence actually diff).
func scorerOutputsEqual(a, b IssueOpsBenchmarkScore) bool {
	return reflect.DeepEqual(a.DimensionScores, b.DimensionScores) &&
		reflect.DeepEqual(a.DeterministicFailures, b.DeterministicFailures) &&
		reflect.DeepEqual(a.CriticalFailures, b.CriticalFailures)
}

func TestScorerDeterminismOrderedOutputsStable(t *testing.T) {
	// A partially-FAILING artifact so the failure slices are non-empty and their
	// ordering is actually exercised (an all-pass artifact would compare empty
	// slices and prove little).
	fixture := issueopscontract.IssueOpsBenchmarkFixture{
		ID:                 "determinism",
		PioneerSkillTarget: "database-design",
		ExpectedRouting:    []issueopscontract.SkillRouting{{Phase: "plan", Skill: "database-design"}},
		CriticalFailures:   []string{"skips domain contract evidence", "skips live evidence matrix"},
	}
	artifact := completeBenchmarkArtifactForTest()
	artifact.PioneerSkillEvidence = coddKeywordEvidence
	artifact.RoutingTrace = []issueopscontract.SkillRouting{{Phase: "plan", Skill: "database-design"}}
	artifact.DomainContractEvidence = ""
	artifact.LiveEvidenceMatrix = ""
	artifact.PhaseChoices = ""

	first := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if len(first.DeterministicFailures) == 0 || len(first.CriticalFailures) == 0 {
		t.Fatalf("determinism fixture must produce non-empty failure slices to exercise ordering: %+v", first)
	}

	const reps = 25
	avgs := make([]float64, 0, reps)
	for range reps {
		got := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
		if !scorerOutputsEqual(first, got) {
			t.Fatalf("scorer is nondeterministic across repeats:\n first=%+v\n got=%+v", first, got)
		}
		avgs = append(avgs, got.AverageScore)
	}
	// Acceptance (b) literally: the score interval collapses to a point.
	if _, _, w := ScoreSpread(avgs); w != 0 {
		t.Fatalf("deterministic gate score spread must be width 0, got %v", w)
	}
}

// Positive control (teeth): the determinism comparison MUST flag a permuted
// failure ordering as non-equal. Without this, the stability test above could
// be passing vacuously (e.g. if the comparison ever weakened to set equality, a
// future refactor that ranged the checks map and scrambled order would slip
// through). This proves the comparison discriminates ordering.
func TestScorerDeterminismComparisonCatchesReordering(t *testing.T) {
	fixture := issueopscontract.IssueOpsBenchmarkFixture{
		ID:               "determinism-teeth",
		CriticalFailures: []string{"skips domain contract evidence", "skips live evidence matrix"},
	}
	artifact := completeBenchmarkArtifactForTest()
	artifact.DomainContractEvidence = ""
	artifact.LiveEvidenceMatrix = ""
	artifact.PhaseChoices = ""

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if len(score.DeterministicFailures) < 2 {
		t.Fatalf("need >=2 deterministic failures to test reordering, got %v", score.DeterministicFailures)
	}

	reordered := score
	reordered.DeterministicFailures = append([]string(nil), score.DeterministicFailures...)
	reordered.DeterministicFailures[0], reordered.DeterministicFailures[1] = reordered.DeterministicFailures[1], reordered.DeterministicFailures[0]
	if scorerOutputsEqual(score, reordered) {
		t.Fatal("determinism comparison must catch reordered DeterministicFailures (otherwise the stability test is vacuous)")
	}
}
