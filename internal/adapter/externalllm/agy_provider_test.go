package externalllm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewAgyProviderDefaults(t *testing.T) {
	p := NewAgyProvider()
	if p.Command != "agy" {
		t.Errorf("expected default command agy, got %q", p.Command)
	}
	if p.Timeout <= 0 {
		t.Errorf("expected positive timeout, got %v", p.Timeout)
	}
}

func TestAgyProviderCommandFallback(t *testing.T) {
	p := &AgyProvider{}
	if p.command() != "agy" {
		t.Errorf("expected fallback agy, got %q", p.command())
	}
	p.Command = "  "
	if p.command() != "agy" {
		t.Errorf("expected fallback agy for whitespace command, got %q", p.command())
	}
}

func TestAgyProviderTimeoutFallback(t *testing.T) {
	p := &AgyProvider{}
	if p.timeout() <= 0 {
		t.Errorf("expected positive default timeout, got %v", p.timeout())
	}
	p.Timeout = 0
	if p.timeout() <= 0 {
		t.Errorf("expected positive fallback timeout, got %v", p.timeout())
	}
}

func TestFormatSchemaExampleEmpty(t *testing.T) {
	if s := formatSchemaExample(nil); s != "{}" {
		t.Errorf("expected {} for nil schema, got %q", s)
	}
	if s := formatSchemaExample(map[string]any{}); s != "{}" {
		t.Errorf("expected {} for empty schema, got %q", s)
	}
}

func TestFormatSchemaExampleWithProps(t *testing.T) {
	schema := map[string]any{
		"properties": map[string]any{
			"ok":      map[string]any{"type": "boolean"},
			"raw":     "loose",
			"score":   map[string]any{"type": "number"},
			"attempt": map[string]any{"type": "integer"},
			"summary": map[string]any{"type": "string"},
			"tags":    map[string]any{"type": "array"},
			"meta":    map[string]any{"type": "object"},
			"unknown": map[string]any{"type": "custom"},
		},
	}
	result := formatSchemaExample(schema)
	if result == "" || result == "{}" {
		t.Errorf("expected non-trivial example, got %q", result)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Fatalf("expected JSON example, got %q: %v", result, err)
	}
	if decoded["ok"] != false {
		t.Errorf("expected boolean example false, got %#v", decoded["ok"])
	}
	if decoded["score"] != float64(0) || decoded["attempt"] != float64(0) {
		t.Errorf("expected numeric examples to be zero, got score=%#v attempt=%#v", decoded["score"], decoded["attempt"])
	}
	if decoded["summary"] != "..." || decoded["raw"] != "..." || decoded["unknown"] != "..." {
		t.Errorf("expected string fallback examples, got %#v", decoded)
	}
	if tags, ok := decoded["tags"].([]any); !ok || len(tags) != 1 || tags[0] != "..." {
		t.Errorf("expected array example, got %#v", decoded["tags"])
	}
	if meta, ok := decoded["meta"].(map[string]any); !ok || meta["..."] != "..." {
		t.Errorf("expected object example, got %#v", decoded["meta"])
	}
}

func TestAgyProviderImplementsExternalLLM(t *testing.T) {
	// Compile-time check via var _ ensures AgyProvider satisfies the interface.
	// This test just confirms the package compiles.
	var p interface{} = NewAgyProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestAgyProviderQueryRunsConfiguredCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake command is POSIX-specific")
	}
	tmp := t.TempDir()
	script := writeAgyFakeCommand(t, tmp, `#!/bin/sh
pwd > cwd.txt
printf '%s\n' "$@" > argv.txt
printf 'raw-output\n'
`)
	p := &AgyProvider{Command: script, WorkDir: tmp, Timeout: 5 * time.Second}

	got, err := p.Query("summarize this")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if got != "raw-output\n" {
		t.Fatalf("unexpected query output %q", got)
	}
	if cwd := strings.TrimSpace(readTestFile(t, filepath.Join(tmp, "cwd.txt"))); cwd != tmp {
		t.Fatalf("expected command cwd %q, got %q", tmp, cwd)
	}
	argv := strings.Split(strings.TrimSpace(readTestFile(t, filepath.Join(tmp, "argv.txt"))), "\n")
	want := []string{"--dangerously-skip-permissions", "-p", "summarize this"}
	if !stringSlicesEqual(argv, want) {
		t.Fatalf("unexpected argv %#v, want %#v", argv, want)
	}
}

func TestAgyProviderQueryStructuredDecodesJSONAndIncludesSchemaPrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake command is POSIX-specific")
	}
	tmp := t.TempDir()
	script := writeAgyFakeCommand(t, tmp, `#!/bin/sh
printf '%s' "$3" > prompt.txt
printf '{"summary":"ok","score":100}'
`)
	p := &AgyProvider{Command: script, WorkDir: tmp, Timeout: 5 * time.Second}
	schema := map[string]any{
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"score":   map[string]any{"type": "integer"},
		},
	}

	got, err := p.QueryStructured("judge result", schema)
	if err != nil {
		t.Fatalf("QueryStructured returned error: %v", err)
	}
	if got["summary"] != "ok" || got["score"] != float64(100) {
		t.Fatalf("unexpected structured result: %#v", got)
	}
	prompt := readTestFile(t, filepath.Join(tmp, "prompt.txt"))
	for _, want := range []string{"judge result", `"summary": "..."`, `"score": 0`} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}

func TestAgyProviderQueryStructuredRejectsMalformedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake command is POSIX-specific")
	}
	tmp := t.TempDir()
	script := writeAgyFakeCommand(t, tmp, `#!/bin/sh
printf 'not json'
`)
	p := &AgyProvider{Command: script, WorkDir: tmp, Timeout: 5 * time.Second}

	_, err := p.QueryStructured("judge result", nil)
	if err == nil {
		t.Fatal("expected malformed output error")
	}
	if !strings.Contains(err.Error(), "agy output must be strict JSON object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeAgyFakeCommand(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "agy-fake")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake command: %v", err)
	}
	return path
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
