package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/cmd/harness/issueopscli/benchmarkartifact"
	issueopscore "agent-harness/internal/adapter/issueops"
)

// 모든 repo fixture에 대해 실제 CLI 디스패치 경로(benchmarkcmd run -> FromFixture
// -> RunIssueOpsBenchmark)를 검증한다. 라이브러리를 직접 호출하면 이 테스트가
// 고정하려는 FromFixture 배선을 건너뛰게 된다.
func TestRunIssueOpsBenchmarkRunCoversPioneerFixturesViaCLI(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixturesPath := filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures")

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", fixturesPath, "--judge", "none", "--json"})
	})

	var result issueopscore.IssueOpsBenchmarkRunResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse benchmark run output: %v\n%s", err, out)
	}
	if result.FixtureCount != 18 {
		t.Fatalf("expected 18 fixtures through the CLI path, got %d", result.FixtureCount)
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
	if pioneerSeen != 13 {
		t.Fatalf("expected 13 targeted fixtures scored via CLI, got %d", pioneerSeen)
	}
}

func TestFromFixturePioneerEvidenceWiring(t *testing.T) {
	target := benchmarkartifact.FromFixture(issueopscore.IssueOpsBenchmarkFixture{ID: "pioneer-dijkstra", PioneerSkillTarget: "dijkstra"})
	if strings.TrimSpace(target.PioneerSkillEvidence) == "" {
		t.Fatal("targeted fixture must produce pioneer evidence")
	}
	nonTarget := benchmarkartifact.FromFixture(issueopscore.IssueOpsBenchmarkFixture{ID: "ambiguous-intent"})
	if nonTarget.PioneerSkillEvidence != "" {
		t.Fatalf("non-targeted fixture must not fabricate pioneer evidence, got %q", nonTarget.PioneerSkillEvidence)
	}
}
