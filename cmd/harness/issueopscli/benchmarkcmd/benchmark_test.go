package benchmarkcmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunBenchmarkRunCompareAndGate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixturesDir := t.TempDir()
	fixturePath := filepath.Join(fixturesDir, "fixture.json")
	if err := os.WriteFile(fixturePath, []byte(`{
		"id":"fixture-1",
		"title":"IssueOps benchmark",
		"user_prompt":"Improve the IssueOps loop",
		"repo_context":"agent-harness Go CLI",
		"expected_issue":["IssueOps"],
		"expected_plan":["Problem intake"],
		"expected_tasks":["Worker A"],
		"expected_tdd":["Write failing tests first"],
		"expected_subagents":["assigned files"],
		"expected_pr":["Verification"],
		"critical_failures":["missing issue"]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveIssueOpsBenchmarkRun(core.StateDir(), core.IssueOpsBenchmarkRunResult{ID: "judge-source", FixtureCount: 1}); err != nil {
		t.Fatalf("save judge source run: %v", err)
	}
	judgePath := filepath.Join(t.TempDir(), "judge.json")
	if err := os.WriteFile(judgePath, []byte(`{
		"source_run_id":"judge-source",
		"provenance":"recorded fresh-context judge",
		"scores":{
			"fixture-1":{
				"ok":true,
				"fixture_id":"fixture-1",
				"average_score":100,
				"minimum_score":100,
				"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"clear"}],
				"passed":true
			}
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"run", "--fixtures", fixturesDir, "--judge", "file", "--judge-file", judgePath, "--json"}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	runs := benchmarkRunIDs(t)
	benchmarkRuns := 0
	for _, id := range runs {
		if strings.HasPrefix(id, "issueops-benchmark-") {
			benchmarkRuns++
		}
	}
	if benchmarkRuns != 1 {
		t.Fatalf("expected one saved benchmark run, got %d (of %d total, incl. the seeded provenance source)", benchmarkRuns, len(runs))
	}
	baseline := benchmarkRunResult("baseline", 90, true)
	candidate := benchmarkRunResult("candidate", 100, true)
	if err := core.SaveIssueOpsBenchmarkRun(core.StateDir(), baseline); err != nil {
		t.Fatalf("save baseline: %v", err)
	}
	if err := core.SaveIssueOpsBenchmarkRun(core.StateDir(), candidate); err != nil {
		t.Fatalf("save candidate: %v", err)
	}
	if err := Run([]string{"compare", "--baseline", baseline.ID, "--candidate", candidate.ID, "--json"}); err != nil {
		t.Fatalf("compare returned error: %v", err)
	}
	candidatePath := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(candidatePath, []byte(`{
		"id":"quality-wave",
		"hypothesis":"More coverage reduces regressions",
		"target_dimensions":["intent_understanding"],
		"edit_surface":["cmd/harness/issueopscli/**"]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"gate", "--baseline", baseline.ID, "--candidate", candidate.ID, "--candidate-file", candidatePath, "--changed-path", "cmd/harness/issueopscli/benchmarkcmd/benchmark.go", "--json"}); err != nil {
		t.Fatalf("gate returned error: %v", err)
	}
}

func TestBenchmarkHelpersAndErrors(t *testing.T) {
	fixtures := []core.IssueOpsBenchmarkFixture{{ID: "known"}}
	scorePath := filepath.Join(t.TempDir(), "scores.json")
	if err := os.WriteFile(scorePath, []byte(`{"source_run_id":"src","provenance":"recorded judge","scores":{"known":{"ok":true,"fixture_id":"known","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"ok"}],"passed":true}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, scores, err := readIssueOpsJudgeMap(scorePath, fixtures)
	if err != nil {
		t.Fatalf("read judge map: %v", err)
	}
	if scores["known"].MinimumScore != 100 {
		t.Fatalf("unexpected score: %#v", scores["known"])
	}
	for name, body := range map[string]string{
		"flat-rejected": `{"known":{"ok":true,"fixture_id":"known","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"ok"}],"passed":true}}`,
		"missing":       `{"source_run_id":"src","provenance":"p","scores":{}}`,
		"unknown":       `{"source_run_id":"src","provenance":"p","scores":{"known":{"ok":true,"fixture_id":"known","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"ok"}],"passed":true},"extra":{}}}`,
		"bad":           `{bad`,
	} {
		path := filepath.Join(t.TempDir(), name+".json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readIssueOpsJudgeMap(path, fixtures); err == nil {
			t.Fatalf("expected %s judge map to fail", name)
		}
	}
	if _, err := readIssueOpsAutoresearchCandidateFile(""); err == nil {
		t.Fatal("empty candidate file should fail")
	}
	badCandidate := filepath.Join(t.TempDir(), "bad-candidate.json")
	if err := os.WriteFile(badCandidate, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readIssueOpsAutoresearchCandidateFile(badCandidate); err == nil {
		t.Fatal("bad candidate JSON should fail")
	}
	candidatePath := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(candidatePath, []byte(`{"id":"c1","hypothesis":"h","target_dimensions":["intent_understanding"],"edit_surface":["cmd/**"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	candidate, err := readIssueOpsAutoresearchCandidateFile(candidatePath)
	if err != nil {
		t.Fatalf("read candidate: %v", err)
	}
	if candidate.ID != "c1" {
		t.Fatalf("candidate ID = %q", candidate.ID)
	}
	var repeated repeatedFlag
	_ = repeated.Set("a")
	_ = repeated.Set("b")
	if repeated.String() != "a,b" {
		t.Fatalf("repeated flag = %q", repeated.String())
	}
	fs := flag.NewFlagSet("help", flag.ContinueOnError)
	help, err := parseFlags(fs, []string{"--help"})
	if !help || err != nil {
		t.Fatalf("expected help without error, got help=%v err=%v", help, err)
	}
	if err := Run(nil); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if err := Run([]string{"run", "--fixtures", t.TempDir(), "--judge", "bad"}); err == nil || !strings.Contains(err.Error(), "no issueops benchmark fixtures") {
		t.Fatalf("expected fixtures error before judge validation, got %v", err)
	}
	if err := Run([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), "unknown issueops benchmark") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestRunBenchmarkReliability(t *testing.T) {
	outcomesPath := filepath.Join(t.TempDir(), "outcomes.json")
	if err := os.WriteFile(outcomesPath, []byte(`{
		"runs":[
			{"run_id":"run-1","provenance":"recorded-holdout","outcomes":{"fixture-a":true,"fixture-b":true}},
			{"run_id":"run-2","provenance":"recorded-holdout","outcomes":{"fixture-a":true,"fixture-b":false}},
			{"run_id":"run-3","provenance":"recorded-holdout","outcomes":{"fixture-a":true,"fixture-b":true}}
		]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"reliability", "--outcomes", outcomesPath, "--json"}); err != nil {
		t.Fatalf("reliability --json returned error: %v", err)
	}
	if err := Run([]string{"reliability", "--outcomes", outcomesPath}); err != nil {
		t.Fatalf("reliability text output returned error: %v", err)
	}

	// The provenance guard must surface as a CLI error (duplicate run_id is a
	// re-scoring of one artifact dressed as two runs).
	dupPath := filepath.Join(t.TempDir(), "dup.json")
	if err := os.WriteFile(dupPath, []byte(`{"runs":[
		{"run_id":"dup","provenance":"p","outcomes":{"a":true}},
		{"run_id":"dup","provenance":"p","outcomes":{"a":false}}
	]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"reliability", "--outcomes", dupPath, "--json"}); err == nil {
		t.Fatal("duplicate run_id must surface as a CLI error")
	}

	badParse := filepath.Join(t.TempDir(), "badparse.json")
	if err := os.WriteFile(badParse, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"reliability", "--outcomes", badParse}); err == nil {
		t.Fatal("malformed outcomes JSON must error")
	}
}

func benchmarkRunIDs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(core.StateDir(), "issueops-benchmarks"))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	return ids
}

func benchmarkRunResult(id string, score float64, ok bool) core.IssueOpsBenchmarkRunResult {
	return core.IssueOpsBenchmarkRunResult{
		OK:                   ok,
		ID:                   id,
		FixtureCount:         1,
		AverageScore:         score,
		MinimumScore:         score,
		CriticalFailureCount: 0,
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           ok,
			FixtureID:    "fixture-1",
			AverageScore: score,
			MinimumScore: score,
			DimensionScores: []core.IssueOpsDimensionScore{{
				Dimension: "intent_understanding",
				Score:     score,
				Evidence:  "evidence",
			}},
			Passed: ok,
		}},
	}
}
