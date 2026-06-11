package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func writeJudgeFileFixtureForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fixture := `{
  "id": "judge-file-fixture",
  "title": "Judge file backend fixture",
  "user_prompt": "judge file 백엔드를 검증해줘",
  "repo_context": "synthetic fixture for the file judge backend",
  "critical_failures": ["works in source repo"]
}`
	if err := os.WriteFile(filepath.Join(dir, "judge-file-fixture.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeJudgeMapForTest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "judge-map.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const judgeFileValidScore = `{"ok":true,"fixture_id":"judge-file-fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`

func TestRunIssueOpsBenchmarkJudgeFileMergesScores(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	judgeFile := writeJudgeMapForTest(t, `{"judge-file-fixture": `+judgeFileValidScore+`}`)

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", judgeFile, "--json"})
	})
	var result core.IssueOpsBenchmarkRunResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out)
	}
	if result.FixtureCount != 1 || result.CriticalFailureCount != 0 {
		t.Fatalf("unexpected run result: %+v", result)
	}
	if len(result.Scores) != 1 || !strings.Contains(result.Scores[0].DimensionScores[0].Evidence, "judge:") {
		t.Fatalf("judge evidence must be merged into dimension scores: %+v", result.Scores)
	}
}

func TestRunIssueOpsBenchmarkJudgeFileFailsClosedOnMissingKey(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	judgeFile := writeJudgeMapForTest(t, `{}`)

	err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", judgeFile, "--json"})
	if err == nil || !strings.Contains(err.Error(), "judge-file-fixture") {
		t.Fatalf("missing fixture key must error before merge, got %v", err)
	}
}

func TestRunIssueOpsBenchmarkJudgeFileFailsClosedOnUnknownKey(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	judgeFile := writeJudgeMapForTest(t, `{"judge-file-fixture": `+judgeFileValidScore+`, "ghost-fixture": `+judgeFileValidScore+`}`)

	err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", judgeFile, "--json"})
	if err == nil || !strings.Contains(err.Error(), "ghost-fixture") {
		t.Fatalf("unknown fixture key must error before merge, got %v", err)
	}
}

func TestRunIssueOpsBenchmarkJudgeFileRejectsNoisyScore(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	judgeFile := writeJudgeMapForTest(t, `{"judge-file-fixture": {"ok":true,"unexpected_field":1}}`)

	err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", judgeFile, "--json"})
	if err == nil {
		t.Fatal("noisy/unknown-field judge score must be rejected by strict decode")
	}
}
