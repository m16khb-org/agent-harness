package webfetchcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFetchJSONReturnsStructuredSafetyRejection(t *testing.T) {
	var stdout bytes.Buffer
	err := RunWithDeps([]string{"fetch", "--url", "http://127.0.0.1/private", "--json"}, Deps{Stdout: &stdout})
	if err != nil {
		t.Fatalf("RunWithDeps returned unexpected error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("fetch JSON did not decode: %v\n%s", err, stdout.String())
	}
	if payload["ok"] != false {
		t.Fatalf("ok=%v, want false", payload["ok"])
	}
	if payload["stop_reason"] != "safety_rejected" {
		t.Fatalf("stop_reason=%v, want safety_rejected", payload["stop_reason"])
	}
}

func TestRunBenchmarkJSONUsesDeterministicFixtures(t *testing.T) {
	var stdout bytes.Buffer
	err := RunWithDeps([]string{"benchmark", "--fixtures", "builtin", "--json"}, Deps{Stdout: &stdout})
	if err != nil {
		t.Fatalf("RunWithDeps returned unexpected error: %v", err)
	}
	var payload struct {
		OK           bool    `json:"ok"`
		Score        float64 `json:"score"`
		FixtureCount int     `json:"fixture_count"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("benchmark JSON did not decode: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Score < 95 || payload.FixtureCount != 12 {
		t.Fatalf("unexpected benchmark payload: %+v", payload)
	}
}

func TestRunBenchmarkJSONLoadsFixtureFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixtures.json")
	if err := os.WriteFile(path, []byte(`{
  "fixtures": [
    {
      "id": "article_basic",
      "status_code": 200,
      "headers": {"Content-Type": "text/html"},
      "body": "<main>`+strings.Repeat("article body ", 60)+`</main>",
      "expected": ["strong_ok"],
      "min_body_chars": 500
    }
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := RunWithDeps([]string{"benchmark", "--fixtures", path, "--json"}, Deps{Stdout: &stdout})
	if err != nil {
		t.Fatalf("RunWithDeps returned unexpected error: %v", err)
	}
	var payload struct {
		OK           bool `json:"ok"`
		FixtureCount int  `json:"fixture_count"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("benchmark JSON did not decode: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.FixtureCount != 1 {
		t.Fatalf("unexpected benchmark payload: %+v", payload)
	}
}

func TestRunBenchmarkLiveRequiresEnvironmentOptIn(t *testing.T) {
	t.Setenv("HARNESS_WEBFETCH_LIVE", "")

	var stdout bytes.Buffer
	err := RunWithDeps([]string{"benchmark", "--fixtures", "builtin", "--live", "--json"}, Deps{Stdout: &stdout})
	if err == nil {
		t.Fatalf("RunWithDeps returned nil error, want live opt-in failure")
	}
	if !strings.Contains(err.Error(), "HARNESS_WEBFETCH_LIVE=1") {
		t.Fatalf("error=%v, want HARNESS_WEBFETCH_LIVE=1 guidance", err)
	}
}

func TestRunBenchmarkLiveUsesGenericBaselineFlag(t *testing.T) {
	t.Setenv("HARNESS_WEBFETCH_LIVE", "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<main>" + strings.Repeat("live article body ", 60) + "</main>"))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "live.json")
	if err := os.WriteFile(path, []byte(`{
  "fixtures": [
    {
      "id": "public_article",
      "url": "`+server.URL+`",
      "expected": ["strong_ok"],
      "min_body_chars": 500
    }
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := RunWithDeps([]string{"benchmark", "--fixtures", path, "--live", "--compare-baseline", filepath.Join(dir, "missing-baseline"), "--json"}, Deps{Stdout: &stdout})
	if err != nil {
		t.Fatalf("RunWithDeps returned unexpected error: %v", err)
	}
	if strings.Contains(strings.ToLower(stdout.String()), "ins"+"ane") {
		t.Fatalf("benchmark output exposed implementation-specific comparator name: %s", stdout.String())
	}
	var payload struct {
		OK                  bool `json:"ok"`
		LiveParityEvaluated bool `json:"live_parity_evaluated"`
		LiveParityReport    struct {
			SuccessRate       float64  `json:"success_rate"`
			BaselineAvailable bool     `json:"baseline_available"`
			Warnings          []string `json:"warnings"`
		} `json:"live_parity_report"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("benchmark JSON did not decode: %v\n%s", err, stdout.String())
	}
	if !payload.LiveParityEvaluated {
		t.Fatalf("unexpected live benchmark payload: %+v", payload)
	}
	if payload.LiveParityReport.BaselineAvailable {
		t.Fatalf("missing baseline command reported available: %+v", payload.LiveParityReport)
	}
}

func TestRunRejectsMissingURL(t *testing.T) {
	err := RunWithDeps([]string{"fetch", "--json"}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "--url is required") {
		t.Fatalf("error=%v, want missing url", err)
	}
}
