package externalllm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunExternalLLMPrintRequiresPrompt(t *testing.T) {
	_, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "   "})
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected prompt error, got err=%v", err)
	}
}

func TestRunExternalLLMPrintRequiresAPIKey(t *testing.T) {
	t.Setenv("Z_AI_API_KEY", "")
	_, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "return json", Timeout: 5 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "Z_AI_API_KEY") {
		t.Fatalf("expected API key error, got err=%v", err)
	}
}

func TestRunExternalLLMPrintCallsZAIWithStructuredJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"key\":\"value\"}"}}]}`))
	}))
	defer ts.Close()

	// Override base URL
	orig := baseURL
	baseURL = ts.URL
	defer func() { baseURL = orig }()

	t.Setenv("Z_AI_API_KEY", "test-key")
	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Prompt:  "return json",
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunExternalLLMPrint() error = %v; output=%s", err, result.Output)
	}
	if string(result.Output) != `{"key":"value"}` {
		t.Fatalf("Output=%q, want {\"key\":\"value\"}", result.Output)
	}
}

func TestRunExternalLLMPrintRejectsLegacyCommandProviderWithoutExecution(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "executed")
	fake := filepath.Join(tmp, "fake-legacy-llm.sh")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\ntouch '"+marker+"'\nprintf 'legacy-output'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Provider: fake,
		Prompt:   "return json",
		Timeout:  5 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported external llm provider") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("legacy command provider was executed; stat marker err=%v", statErr)
	}
}

func TestRunExternalLLMPrintDisableStructuredJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"old-school format"}}]}`))
	}))
	defer ts.Close()

	orig := baseURL
	baseURL = ts.URL
	defer func() { baseURL = orig }()

	t.Setenv("Z_AI_API_KEY", "test-key")
	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Prompt:                "do stuff",
		Timeout:               10 * time.Second,
		DisableStructuredJSON: true,
	})
	if err != nil {
		t.Fatalf("RunExternalLLMPrint() error = %v", err)
	}
	if string(result.Output) != "old-school format" {
		t.Fatalf("Output=%q, want non-structured, unmodified content", result.Output)
	}
}

func TestRunExternalLLMPrintReturnsCommandErrorWithOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"provider-failed"}}`))
	}))
	defer ts.Close()

	orig := baseURL
	baseURL = ts.URL
	defer func() { baseURL = orig }()

	t.Setenv("Z_AI_API_KEY", "test-key")
	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Prompt:  "return json",
		Timeout: 10 * time.Second,
	})
	if err == nil {
		t.Fatalf("expected command error, got result=%+v", result)
	}
	if !strings.Contains(string(result.Output), "provider-failed") {
		t.Fatalf("Output=%q, want provider failure text", result.Output)
	}
}

func TestRunExternalLLMPrintTimeoutKillsProcessGroup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(2 * time.Second)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	orig := baseURL
	baseURL = ts.URL
	defer func() { baseURL = orig }()

	t.Setenv("Z_AI_API_KEY", "test-key")
	started := time.Now()
	_, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Prompt:  "return json",
		Timeout: 50 * time.Millisecond,
	})
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got err=%v", err)
	}
	if elapsed > 750*time.Millisecond {
		t.Fatalf("timeout should not wait for slow server; elapsed=%s", elapsed)
	}
}

func TestExternalLLMPrintCommandPreview(t *testing.T) {
	preview := ExternalLLMPrintCommandPreview()
	if !strings.Contains(preview, "zai:") {
		t.Fatalf("preview %q does not contain zai:", preview)
	}
	if !strings.Contains(preview, "glm-5-turbo") {
		t.Fatalf("preview %q does not contain default model", preview)
	}
}

func TestDefaultModel(t *testing.T) {
	if DefaultModel() != "glm-5-turbo" {
		t.Fatalf("DefaultModel()=%q, want glm-5-turbo", DefaultModel())
	}
}
