package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/cmd/harness/issueopscli/benchmarkartifact"
	"agent-harness/internal/core"
)

// Exercises the REAL CLI dispatch path (benchmarkcmd run -> FromFixture ->
// RunIssueOpsBenchmark) over all repo fixtures. Calling the library directly
// would skip the FromFixture wiring this test exists to pin.
func TestRunIssueOpsBenchmarkRunCoversPioneerFixturesViaCLI(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixturesPath := filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures")

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", fixturesPath, "--judge", "none", "--json"})
	})

	var result core.IssueOpsBenchmarkRunResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse benchmark run output: %v\n%s", err, out)
	}
	if result.FixtureCount != 15 {
		t.Fatalf("expected 15 fixtures through the CLI path, got %d", result.FixtureCount)
	}
	if result.CriticalFailureCount != 0 {
		t.Fatalf("expected zero critical failures, got %d: %+v", result.CriticalFailureCount, result.Scores)
	}
	pioneerSeen := 0
	for _, score := range result.Scores {
		if len(score.DeterministicFailures) != 0 {
			t.Fatalf("fixture %s has deterministic failures: %v", score.FixtureID, score.DeterministicFailures)
		}
		for _, dim := range score.DimensionScores {
			if dim.Dimension != "pioneer_skill_contribution" {
				continue
			}
			if strings.HasPrefix(score.FixtureID, "pioneer-") {
				pioneerSeen++
				if dim.NotApplicable || dim.Score != 100 {
					t.Fatalf("pioneer fixture %s must score 100 on pioneer dimension, got %+v", score.FixtureID, dim)
				}
			} else if !dim.NotApplicable {
				t.Fatalf("non-pioneer fixture %s must record pioneer dimension as N/A, got %+v", score.FixtureID, dim)
			}
		}
	}
	if pioneerSeen != 10 {
		t.Fatalf("expected 10 pioneer fixtures scored via CLI, got %d", pioneerSeen)
	}
}

func TestFromFixturePioneerEvidenceWiring(t *testing.T) {
	target := benchmarkartifact.FromFixture(core.IssueOpsBenchmarkFixture{ID: "pioneer-dijkstra", PioneerSkillTarget: "dijkstra"})
	if strings.TrimSpace(target.PioneerSkillEvidence) == "" {
		t.Fatal("targeted fixture must produce pioneer evidence")
	}
	nonTarget := benchmarkartifact.FromFixture(core.IssueOpsBenchmarkFixture{ID: "ambiguous-intent"})
	if nonTarget.PioneerSkillEvidence != "" {
		t.Fatalf("non-targeted fixture must not fabricate pioneer evidence, got %q", nonTarget.PioneerSkillEvidence)
	}
}
