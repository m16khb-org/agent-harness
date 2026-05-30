package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hookTempRepoWithDoc(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "ARCHITECTURE.md"), []byte("# Arch\n\n## 경계\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func runHookCapture(t *testing.T, stdinJSON string, fn func() error) map[string]any {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() { _, _ = io.WriteString(w, stdinJSON); _ = w.Close() }()
	defer func() { os.Stdin = oldStdin }()
	out := captureStdoutForTest(t, func() {
		if err := fn(); err != nil {
			t.Fatalf("hook: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook output is not JSON: %q: %v", out, err)
	}
	return obj
}

func hookAdditionalContext(obj map[string]any) string {
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso == nil {
		return ""
	}
	ctx, _ := hso["additionalContext"].(string)
	return ctx
}

func TestRunHookUserPromptDropsCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"prompt":"x","cwd":"`+repo+`"}`, func() error { return runHookUserPrompt(nil) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("user-prompt must not carry a catalog systemMessage: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); strings.Contains(ctx, "project docs (read what's relevant):") || strings.Contains(ctx, "📚") {
		t.Fatalf("user-prompt must not inject the project-doc catalog: %q", ctx)
	}
}

func TestRunHookSessionStartInjectsCatalogClaude(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHookSessionStart(nil) })
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
		t.Fatalf("SessionStart should show the pretty catalog via systemMessage: %v", obj["systemMessage"])
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("SessionStart should inject the compact catalog additionalContext: %q", ctx)
	}
}

func TestRunHookSessionStartCodexOmitsSystemMessage(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHookSessionStart([]string{"--host", "codex"}) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("Codex SessionStart must omit systemMessage: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "• ARCHITECTURE.md") || strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("Codex SessionStart additionalContext should be the readable catalog view: %q", ctx)
	}
}

func TestRunHookSessionStartSkipsOnCompactSource(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"compact"}`, func() error { return runHookSessionStart(nil) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("compact-source SessionStart should not inject (PostCompact owns it): %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); ctx != "" {
		t.Fatalf("compact-source SessionStart should emit no additionalContext: %q", ctx)
	}
}

func TestRunHookPostCompactInjectsCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPostCompact(nil) })
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("PostCompact should re-inject the catalog after compaction: %q", ctx)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") {
		t.Fatalf("PostCompact (claude) should show the pretty catalog via systemMessage: %v", obj["systemMessage"])
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
