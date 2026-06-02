package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIssueOpsAgyJudgeParsesStrictJSON(t *testing.T) {
	fake := writeFakeIssueOpsAgy(t, `{"ok":true,"fixture_id":"fixture","average_score":5,"minimum_score":5,"dimension_scores":[{"dimension":"intent_understanding","score":5,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`)
	result, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
		Artifact:   IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	})
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("unexpected judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsAgyJudgeRejectsNoisyOutput(t *testing.T) {
	fake := writeFakeIssueOpsAgy(t, `I will judge now. {"ok":true}`)
	_, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err == nil {
		t.Fatal("expected strict JSON error")
	}
}

func writeFakeIssueOpsAgy(t *testing.T, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agy.sh")
	script := "#!/bin/sh\nif [ \"$1\" != \"-p\" ]; then echo missing -p >&2; exit 2; fi\ncat <<'EOF'\n" + output + "\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
