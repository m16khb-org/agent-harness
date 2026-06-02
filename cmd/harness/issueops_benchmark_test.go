package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunIssueOpsBenchmarkCLI(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "testdata", "issueops", "fixtures"), "--judge", "none", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["fixture_count"].(float64) < 1 {
		t.Fatalf("unexpected benchmark result: %#v", result)
	}
}

func TestRunIssueOpsBenchmarkCLIAgyKeepsOneScorePerFixture(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fakeAgy := filepath.Join(t.TempDir(), "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
cat <<'EOF'
{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"judge ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "testdata", "issueops", "fixtures"), "--judge", "agy", "--agy-command", fakeAgy, "--json"})
	})
	var result struct {
		OK           bool             `json:"ok"`
		FixtureCount int              `json:"fixture_count"`
		Scores       []map[string]any `json:"scores"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if !result.OK || len(result.Scores) != result.FixtureCount {
		t.Fatalf("expected one merged score per fixture: %+v", result)
	}
}
