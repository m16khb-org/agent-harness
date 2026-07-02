package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/externalllm"
)

func writeFileForUsageTest(path string) error {
	return os.WriteFile(path, []byte("occupied"), 0o644)
}

func withCoreUsageFakeZAI(t *testing.T, body string) {
	t.Helper()
	t.Setenv("Z_AI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	previous := externalllm.SetBaseURL(server.URL)
	t.Cleanup(func() { externalllm.SetBaseURL(previous) })
}

func TestExternalLLMUsageObservationWritesStateRecord(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	withCoreUsageFakeZAI(t, `{"choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`)

	if _, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "observe me"}); err != nil {
		t.Fatalf("run: %v", err)
	}

	list, err := StateList()
	if err != nil {
		t.Fatalf("state list: %v", err)
	}
	var usageKey string
	for _, key := range list.Keys {
		if strings.HasPrefix(key, "external-llm-usage-") {
			usageKey = key
		}
	}
	if usageKey == "" {
		t.Fatalf("no external-llm-usage-* record written; keys = %v", list.Keys)
	}
	record, err := StateRead(usageKey)
	if err != nil {
		t.Fatalf("state read: %v", err)
	}
	var snapshot struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Provider      string `json:"provider"`
		Model         string `json:"model"`
		TotalTokens   int    `json:"total_tokens"`
		DurationMS    int64  `json:"duration_ms"`
		OK            bool   `json:"ok"`
		GeneratedAt   string `json:"generated_at"`
	}
	if err := json.Unmarshal([]byte(record.Record.Content), &snapshot); err != nil {
		t.Fatalf("parse usage record: %v", err)
	}
	if snapshot.Kind != "external_llm_usage" || snapshot.Provider != "zai" {
		t.Errorf("record kind/provider = %q/%q", snapshot.Kind, snapshot.Provider)
	}
	if snapshot.TotalTokens != 18 || !snapshot.OK || snapshot.GeneratedAt == "" {
		t.Errorf("record = %+v, want total 18, ok, timestamped", snapshot)
	}
}

func TestExternalLLMUsageRecordingFailureDoesNotBlockCall(t *testing.T) {
	// Point the state dir at a regular file so every StateWrite fails.
	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := writeFileForUsageTest(blocked); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARNESS_STATE_DIR", blocked)
	withCoreUsageFakeZAI(t, `{"choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)

	if _, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "must not block"}); err != nil {
		t.Fatalf("LLM call must succeed even when usage recording fails: %v", err)
	}
}
