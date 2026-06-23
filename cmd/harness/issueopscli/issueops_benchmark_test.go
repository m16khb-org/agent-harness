package issueopscli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/externalllm"
)

func TestRunIssueOpsBenchmarkCLI(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures"), "--judge", "none", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["fixture_count"].(float64) < 1 {
		t.Fatalf("unexpected benchmark result: %#v", result)
	}
}

func TestRunIssueOpsBenchmarkCLILLMKeepsOneScorePerFixture(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	withFakeIssueOpsCLIZAI(t, `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"judge ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`)

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures"), "--judge", "llm", "--model", "glm-5-turbo", "--json"})
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

func withFakeIssueOpsCLIZAI(t *testing.T, content string) {
	t.Helper()
	t.Setenv("Z_AI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, content)
	}))
	t.Cleanup(server.Close)
	previous := externalllm.SetBaseURL(server.URL)
	t.Cleanup(func() { externalllm.SetBaseURL(previous) })
}
