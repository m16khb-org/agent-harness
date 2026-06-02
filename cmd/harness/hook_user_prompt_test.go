package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestRunHookRecordsFailureEvent(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	_, _ = io.WriteString(w, `{"cwd":"/repo","tool_name":"Bash","tool_input":{"command":"echo TOKEN=secret-value && rg hook cmd"}}`)
	_ = w.Close()
	defer func() { os.Stdin = oldStdin }()

	err = runHook([]string{"unknown-hook", "--token=secret-value"})
	if err == nil {
		t.Fatal("runHook() error = nil")
	}
	body, readErr := os.ReadFile(filepath.Join(stateDir, "hook-failures.jsonl"))
	if readErr != nil {
		t.Fatalf("read hook failure log: %v", readErr)
	}
	text := string(body)
	if !strings.Contains(text, `"hook":"unknown-hook"`) || !strings.Contains(text, "unknown hook subcommand") {
		t.Fatalf("hook failure log missing event details: %s", text)
	}
	if !strings.Contains(text, `"tool":"Bash"`) || !strings.Contains(text, "command_snippet") {
		t.Fatalf("hook failure log missing payload details: %s", text)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatalf("hook failure log leaked secret: %s", text)
	}
}

func TestRunHookFailuresJSON(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if err := os.WriteFile(filepath.Join(stateDir, "hook-failures.jsonl"), []byte(`{"timestamp":"2026-06-02T00:00:00Z","hook":"pre-tool-use","error":"failed"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForTest(t, func() {
		if err := runHook([]string{"failures", "--json"}); err != nil {
			t.Fatalf("runHook failures: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook failures output is not JSON: %q: %v", out, err)
	}
	if obj["ok"] != true || obj["path"] == "" {
		t.Fatalf("unexpected hook failures output: %+v", obj)
	}
	events, _ := obj["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %+v", obj)
	}
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

func TestRunHookUserPromptAgyHintsAreOptIn(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	input := `{"prompt":"이 계획을 검토하고 개선점을 분석해줘","cwd":"` + repo + `"}`
	disabled := runHookCapture(t, input, func() error { return runHookUserPrompt(nil) })
	if strings.Contains(hookAdditionalContext(disabled), "agy -p") {
		t.Fatalf("agy hint should be disabled by default: %q", hookAdditionalContext(disabled))
	}
	enabled := runHookCapture(t, input, func() error { return runHookUserPrompt([]string{"--enable-agy-hints"}) })
	if !strings.Contains(hookAdditionalContext(enabled), "agy -p for LLM second-pass review") {
		t.Fatalf("agy hint should be enabled by flag: %q", hookAdditionalContext(enabled))
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

func TestRunHookPostCompactCodexEmitsCompatibleSchema(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPostCompact([]string{"--host", "codex"}) })
	if _, ok := obj["hookSpecificOutput"]; ok {
		t.Fatalf("Codex PostCompact must not emit hookSpecificOutput: %+v", obj)
	}
	if _, ok := obj["additionalContext"]; ok {
		t.Fatalf("Codex PostCompact must not emit additionalContext: %+v", obj)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
		t.Fatalf("Codex PostCompact should use supported systemMessage catalog: %v", obj["systemMessage"])
	}
}

func TestRunHookPreCompactEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPreCompact(nil) })
	if len(obj) != 0 {
		t.Fatalf("PreCompact hook host output must be a no-op object, got %+v", obj)
	}
}

func TestRunHookPreToolUseEmitsCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"x.go"}}`, func() error { return runHookPreToolUse(nil) })
	if len(obj) != 0 {
		t.Fatalf("PreToolUse hook host output must be a no-op object, got %+v", obj)
	}
}

func TestRunHookPreToolUseRawJSONIsAllowByDefault(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"x.go"}}`, func() error {
		return runHookPreToolUse([]string{"--json"})
	})
	if obj["decision"] != "allow" || obj["source"] != "pre-tool-use" || obj["tool"] != "Edit" {
		t.Fatalf("unexpected PreToolUse raw result: %+v", obj)
	}
}

func TestRunHookPreToolUseEnforcesSearchRoutingForStructuralSourceSearch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg -n \"func Run\" cmd internal"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "block" || obj["tool"] != "Bash" {
		t.Fatalf("expected enforced source search to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "CodeGraph") || !strings.Contains(reason, "codegraph_context") {
		t.Fatalf("block reason should point agents to CodeGraph, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsLiteralEvidenceSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg \"response_contracts\" cmd/harness/testdata README.md"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected docs/golden literal search to remain allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseHostJSONBlocksWhenSearchRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"grep -R \"type Hook\" internal/core"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected host hook JSON to block enforced source search, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsExternalSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"grep -R \"PostToolUse\" -n /Applications/Codex.app/Contents/Resources"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected external binary/app search to remain allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseClaudeHostUsesPermissionDecisionWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg \"type Hook\" internal/core"}}`, func() error {
		return runHookPreToolUse([]string{"--host", "claude", "--enforce-search-routing"})
	})
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PreToolUse" || hso["permissionDecision"] != "deny" {
		t.Fatalf("expected Claude PreToolUse permission denial, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksCodeGraphForExactSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"codegraph_context","tool_input":{"query":"DATABASE_URL"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected exact CodeGraph query to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "Use rg first") {
		t.Fatalf("block reason should point agents to rg, got %q", reason)
	}
}

func TestRunHookPostToolUseQueuesDraftWikiAndWorkerWritesDraft(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	configPath := filepath.Join(repo, "agy-settings.json")
	if err := os.WriteFile(configPath, []byte(`{"model":"Gemini 3.5 Flash (High)"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeAgy := filepath.Join(repo, "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
if [ "$1" != "-p" ]; then
  echo "missing -p" >&2
  exit 2
fi
cat <<'EOF'
---
title: "Hook queued draft"
source: "claude-mem"
target_wiki: "agent-harness"
target_type: "notes"
summary: "PostToolUse hooks queue draft-wiki work for the worker."
---

# Hook queued draft

The hook records a queue item and the worker calls agy -p to produce this draft.
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}

	raw := runHookCapture(t, `{
  "cwd": "`+repo+`",
  "tool_name": "Bash",
  "tool_input": {"command": "claude-mem export observations"},
  "tool_response": "A durable observation says PostToolUse must enqueue draft wiki work while agy runs in a worker."
}`, func() error {
		return runHookPostToolUse([]string{"--json"})
	})
	queue, _ := raw["draft_wiki_queue"].(map[string]any)
	if queue == nil || queue["path"] == "" {
		t.Fatalf("PostToolUse raw JSON did not expose draft_wiki_queue: %+v", raw)
	}

	out := captureStdoutForTest(t, func() {
		if err := runWorker([]string{"draft-wiki", "--repo", repo, "--agy-command", fakeAgy, "--agy-settings", configPath, "--json"}); err != nil {
			t.Fatalf("worker draft-wiki: %v", err)
		}
	})
	var processed map[string]any
	if err := json.Unmarshal([]byte(out), &processed); err != nil {
		t.Fatalf("worker output is not JSON: %q: %v", out, err)
	}
	if processed["processed"] != float64(1) || processed["succeeded"] != float64(1) {
		t.Fatalf("worker did not process queued draft-wiki item: %+v", processed)
	}
	wantDraftName := time.Now().Format(time.DateOnly) + "-hook-queued-draft.md"
	if _, err := os.Stat(filepath.Join(repo, ".agent-harness", "draft-wiki", "draft", wantDraftName)); err != nil {
		t.Fatalf("draft-wiki draft file missing after hook+worker: %v", err)
	}
}

func TestRunHookPostToolUseEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"true"},"tool_response":"ok"}`, func() error {
		return runHookPostToolUse(nil)
	})
	if len(obj) != 0 {
		t.Fatalf("PostToolUse host output must be a no-op object, got %+v", obj)
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
