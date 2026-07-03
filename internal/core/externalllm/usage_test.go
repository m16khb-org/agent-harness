package externalllm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func withUsageFakeZAI(t *testing.T, body string) {
	t.Helper()
	t.Setenv("Z_AI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	previous := SetBaseURL(server.URL)
	t.Cleanup(func() { SetBaseURL(previous) })
}

func TestRunExternalLLMPrintParsesUsageAndNotifiesRecorder(t *testing.T) {
	withUsageFakeZAI(t, `{"choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":120,"completion_tokens":30,"total_tokens":150}}`)

	var observed []ExternalLLMUsageObservation
	previous := SetUsageRecorder(func(obs ExternalLLMUsageObservation) {
		observed = append(observed, obs)
	})
	t.Cleanup(func() { SetUsageRecorder(previous) })

	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "measure me"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Usage == nil {
		t.Fatal("result must carry the parsed usage block")
	}
	if result.Usage.PromptTokens != 120 || result.Usage.CompletionTokens != 30 || result.Usage.TotalTokens != 150 {
		t.Errorf("usage = %+v, want 120/30/150", result.Usage)
	}
	if result.Model == "" {
		t.Error("result must carry the resolved model name")
	}
	if result.DurationMS < 0 {
		t.Errorf("duration_ms = %d, want >= 0", result.DurationMS)
	}

	if len(observed) != 1 {
		t.Fatalf("recorder observations = %d, want exactly 1", len(observed))
	}
	obs := observed[0]
	if obs.Provider != "zai" || obs.Model != DefaultModel() {
		t.Errorf("observation provider/model = %q/%q, want zai/%s", obs.Provider, obs.Model, DefaultModel())
	}
	if obs.Usage == nil || obs.Usage.TotalTokens != 150 {
		t.Errorf("observation usage = %+v, want total 150", obs.Usage)
	}
	if !obs.OK {
		t.Error("observation for a successful call must be OK")
	}
}

func TestRunZAIUsageObservationUsesRequestProvider(t *testing.T) {
	withUsageFakeZAI(t, `{"choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)

	var observed []ExternalLLMUsageObservation
	previous := SetUsageRecorder(func(obs ExternalLLMUsageObservation) {
		observed = append(observed, obs)
	})
	t.Cleanup(func() { SetUsageRecorder(previous) })

	_, err := runZAI(ExternalLLMPrintRequest{Provider: "zai-coding-plan", Prompt: "measure provider"}, time.Second)
	if err != nil {
		t.Fatalf("runZAI: %v", err)
	}
	if len(observed) != 1 {
		t.Fatalf("recorder observations = %d, want exactly 1", len(observed))
	}
	if observed[0].Provider != "zai-coding-plan" {
		t.Fatalf("observation provider = %q, want request provider", observed[0].Provider)
	}
}

func TestRunExternalLLMPrintWithoutRecorderStillSucceeds(t *testing.T) {
	withUsageFakeZAI(t, `{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`)

	previous := SetUsageRecorder(nil)
	t.Cleanup(func() { SetUsageRecorder(previous) })

	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "no recorder"})
	if err != nil {
		t.Fatalf("run without recorder: %v", err)
	}
	if result.Usage != nil {
		t.Errorf("usage = %+v, want nil when the response has no usage block", result.Usage)
	}
}

func TestRunExternalLLMPrintNotifiesRecorderOnHTTPFailure(t *testing.T) {
	t.Setenv("Z_AI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	t.Cleanup(server.Close)
	previousURL := SetBaseURL(server.URL)
	t.Cleanup(func() { SetBaseURL(previousURL) })

	var observed []ExternalLLMUsageObservation
	previousRecorder := SetUsageRecorder(func(obs ExternalLLMUsageObservation) {
		observed = append(observed, obs)
	})
	t.Cleanup(func() { SetUsageRecorder(previousRecorder) })

	_, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "measure failed call"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("expected HTTP failure, got %v", err)
	}
	if len(observed) != 1 {
		t.Fatalf("recorder observations = %d, want exactly 1", len(observed))
	}
	obs := observed[0]
	if obs.OK {
		t.Fatal("failed HTTP call observation must set OK=false")
	}
	if obs.Provider != "zai" || obs.Model != DefaultModel() {
		t.Fatalf("observation provider/model = %q/%q, want zai/%s", obs.Provider, obs.Model, DefaultModel())
	}
	if obs.DurationMS < 0 {
		t.Fatalf("duration_ms = %d, want >= 0", obs.DurationMS)
	}
	if obs.Usage != nil {
		t.Fatalf("HTTP failure usage = %+v, want nil when no usage block is parsed", obs.Usage)
	}
}

func TestRunExternalLLMPrintNotifiesRecorderOnAPIErrorWithUsage(t *testing.T) {
	withUsageFakeZAI(t, `{"error":{"message":"provider refused","code":"rate_limit"},"usage":{"prompt_tokens":11,"completion_tokens":0,"total_tokens":11}}`)

	var observed []ExternalLLMUsageObservation
	previous := SetUsageRecorder(func(obs ExternalLLMUsageObservation) {
		observed = append(observed, obs)
	})
	t.Cleanup(func() { SetUsageRecorder(previous) })

	_, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "measure api error"})
	if err == nil || !strings.Contains(err.Error(), "provider refused") {
		t.Fatalf("expected API error, got %v", err)
	}
	if len(observed) != 1 {
		t.Fatalf("recorder observations = %d, want exactly 1", len(observed))
	}
	obs := observed[0]
	if obs.OK {
		t.Fatal("failed API call observation must set OK=false")
	}
	if obs.Usage == nil || obs.Usage.TotalTokens != 11 {
		t.Fatalf("observation usage = %+v, want total 11", obs.Usage)
	}
	if obs.DurationMS < 0 {
		t.Fatalf("duration_ms = %d, want >= 0", obs.DurationMS)
	}
}
