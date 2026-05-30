package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHookUserPromptEmitsSystemMessageAndContext(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "ARCHITECTURE.md"), []byte("# Arch\n\n## 경계\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() { _, _ = io.WriteString(w, `{"prompt":"x","cwd":"`+repo+`"}`); _ = w.Close() }()
	defer func() { os.Stdin = oldStdin }()

	out := captureStdoutForTest(t, func() {
		if err := runHookUserPrompt(nil); err != nil {
			t.Fatalf("runHookUserPrompt: %v", err)
		}
	})

	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook output is not JSON: %q: %v", out, err)
	}
	sysMsg, _ := obj["systemMessage"].(string)
	if !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
		t.Fatalf("expected pretty user-visible systemMessage, got: %v", obj["systemMessage"])
	}
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso == nil {
		t.Fatalf("missing hookSpecificOutput: %s", out)
	}
	if ctx, _ := hso["additionalContext"].(string); !strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("expected compact model additionalContext, got: %v", hso["additionalContext"])
	}
}

func TestRunHookStopEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	out := captureStdoutForTest(t, func() {
		if err := runHookStop([]string{"--repo", repo}); err != nil {
			t.Fatalf("runHookStop: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("stop hook output is not JSON: %q: %v", out, err)
	}
	if len(obj) != 0 {
		t.Fatalf("Stop hook host output must be a no-op object, got %s", out)
	}
	if strings.Contains(out, "hookSpecificOutput") || strings.Contains(out, "additionalContext") {
		t.Fatalf("Stop hook output contains unsupported injection fields: %s", out)
	}
}

func captureStdoutForTest(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
