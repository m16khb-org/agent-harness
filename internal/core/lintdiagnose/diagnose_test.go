package lintdiagnose

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-harness/internal/core/externalllm"
)

func TestDiagnoseCommandRejectsEmptyDiagnosis(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"diagnosis\":\"\"}"}}]}`))
	}))
	defer ts.Close()

	origBaseURL := externalllm.SetBaseURL(ts.URL)
	defer externalllm.SetBaseURL(origBaseURL)

	t.Setenv("Z_AI_API_KEY", "test-key")

	root := t.TempDir()

	result, err := DiagnoseCommand(LintDiagnoseRequest{
		RepoRoot:    root,
		CommandArgv: []string{"/bin/sh", "-c", "echo lint failed >&2; exit 2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Failed {
		t.Fatalf("expected failed command result: %+v", result)
	}
	if !strings.Contains(result.Diagnosis, "missing diagnosis") {
		t.Fatalf("expected parsing diagnostic for empty diagnosis, got %q", result.Diagnosis)
	}
}
