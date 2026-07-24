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

// writeProvenancedJudgeMap는 별개의 source run을 state dir에 저장하고 주어진
// scores 객체를 provenance 붙은 judge-map 형식으로 감싸, provenance 게이트
// (source_run_id가 실재하는 다른 run으로 해석됨)를 통과하게 한다.
func writeProvenancedJudgeMap(t *testing.T, scoresJSON string) string {
	t.Helper()
	source := core.IssueOpsBenchmarkRunResult{ID: "judge-source-run", FixtureCount: 1}
	if err := core.SaveIssueOpsBenchmarkRun(core.StateDir(), source); err != nil {
		t.Fatal(err)
	}
	return writeJudgeMapForTest(t, `{"source_run_id":"judge-source-run","provenance":"recorded fresh-context judge","scores":`+scoresJSON+`}`)
}

const judgeFileValidScore = `{"ok":true,"fixture_id":"judge-file-fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`

func TestRunIssueOpsBenchmarkJudgeFileMergesScores(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	judgeFile := writeProvenancedJudgeMap(t, `{"judge-file-fixture": `+judgeFileValidScore+`}`)

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
	judgeFile := writeProvenancedJudgeMap(t, `{}`)

	err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", judgeFile, "--json"})
	if err == nil || !strings.Contains(err.Error(), "judge-file-fixture") {
		t.Fatalf("missing fixture key must error before merge, got %v", err)
	}
}

func TestRunIssueOpsBenchmarkJudgeFileFailsClosedOnUnknownKey(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	judgeFile := writeProvenancedJudgeMap(t, `{"judge-file-fixture": `+judgeFileValidScore+`, "ghost-fixture": `+judgeFileValidScore+`}`)

	err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", judgeFile, "--json"})
	if err == nil || !strings.Contains(err.Error(), "ghost-fixture") {
		t.Fatalf("unknown fixture key must error before merge, got %v", err)
	}
}

func TestRunIssueOpsBenchmarkJudgeFileRejectsNoisyScore(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	judgeFile := writeProvenancedJudgeMap(t, `{"judge-file-fixture": {"ok":true,"unexpected_field":1}}`)

	err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", judgeFile, "--json"})
	if err == nil {
		t.Fatal("noisy/unknown-field judge score must be rejected by strict decode")
	}
}

// 감싼 형식만 유일하게 허용되는 형태다. legacy flat judge map과 source_run_id가
// 빠진 wrapper는 둘 다 거부되어야 하며, 그래야 형식 선택으로 provenance를 몰래
// 우회할 수 없다.
func TestRunIssueOpsBenchmarkJudgeFileRejectsUnprovenancedMaps(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)

	flat := writeJudgeMapForTest(t, `{"judge-file-fixture": `+judgeFileValidScore+`}`)
	if err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", flat, "--json"}); err == nil {
		t.Fatal("legacy flat judge map must be rejected (no silent provenance bypass)")
	}

	noSource := writeJudgeMapForTest(t, `{"provenance":"x","scores":{"judge-file-fixture": `+judgeFileValidScore+`}}`)
	if err := runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "file", "--judge-file", noSource, "--json"}); err == nil || !strings.Contains(err.Error(), "source_run_id") {
		t.Fatalf("missing source_run_id must be rejected by the provenance gate, got %v", err)
	}
}

// 결정론적 headline 게이트(--judge none)는 provenance 의존성을 가져서는 안 된다.
// judge file 없이 실행되며 반드시 성공해야 한다.
func TestRunIssueOpsBenchmarkJudgeNoneNeedsNoProvenance(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixtures := writeJudgeFileFixtureForTest(t)
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", fixtures, "--judge", "none", "--json"})
	})
	var result core.IssueOpsBenchmarkRunResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out)
	}
	if result.FixtureCount != 1 {
		t.Fatalf("--judge none must run without any judge map: %+v", result)
	}
}
