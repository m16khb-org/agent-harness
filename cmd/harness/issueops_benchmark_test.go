package main

import (
	"encoding/json"
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
