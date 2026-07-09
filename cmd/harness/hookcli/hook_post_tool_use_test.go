package hookcli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunHookPostToolUseInjectsSourceCheckoutMisdirectWarningOnClaude(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, repo, "2519-test-quality-comprehensive")
	if _, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), cycle.id, string(core.IssueOpsPhaseImplement)); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repo, "src", "a.ts")
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"apply_patch","tool_input":{"file_path":"`+target+`"}}`, func() error {
		return runHookPostToolUse([]string{"--host", "claude"})
	})
	ctx := hookAdditionalContext(obj)
	for _, want := range []string{cycle.id, cycle.path, "소스 체크아웃", "의도한 대상인지 확인"} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("PostToolUse warning missing %q in context %q (obj=%+v)", want, ctx, obj)
		}
	}
}

// B3: a .go edit that leaves an unformatted file injects a deterministic gofmt
// feedback as additionalContext on Claude.
func TestRunHookPostToolUseInjectsLintFeedbackOnClaude(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bad := filepath.Join(repo, "bad.go")
	if err := os.WriteFile(bad, []byte("package p\nfunc F()  {\nreturn\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"`+bad+`"}}`, func() error {
		return runHookPostToolUse([]string{"--host", "claude"})
	})
	ctx := hookAdditionalContext(obj)
	if !strings.Contains(ctx, "bad.go") || !strings.Contains(ctx, "gofmt") {
		t.Fatalf("claude PostToolUse must inject gofmt feedback naming the file, got %q (obj=%+v)", ctx, obj)
	}
}

func TestRunHookPostToolUseCleanEditNoFeedback(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	good := filepath.Join(repo, "good.go")
	if err := os.WriteFile(good, []byte("package p\n\nfunc F() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"`+good+`"}}`, func() error {
		return runHookPostToolUse([]string{"--host", "claude"})
	})
	if len(obj) != 0 {
		t.Fatalf("clean .go edit must produce a no-op object, got %+v", obj)
	}
}

// Codex gets NO --host, so even a lint failure must keep the no-op shape (Codex
// rejects PostToolUse additionalContext as invalid hook JSON).
func TestRunHookPostToolUseCodexStaysNoopOnLintFailure(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bad := filepath.Join(repo, "bad.go")
	if err := os.WriteFile(bad, []byte("package p\nfunc F()  {\nreturn\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"`+bad+`"}}`, func() error {
		return runHookPostToolUse(nil)
	})
	if len(obj) != 0 {
		t.Fatalf("codex (no --host) must stay no-op even on lint failure, got %+v", obj)
	}
}

func TestRunHookPostToolUseSkipsNonGoEdit(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	md := filepath.Join(repo, "README.md")
	if err := os.WriteFile(md, []byte("# x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"`+md+`"}}`, func() error {
		return runHookPostToolUse([]string{"--host", "claude"})
	})
	if len(obj) != 0 {
		t.Fatalf("non-Go edit must not be linted (no-op), got %+v", obj)
	}
}

func TestRunHookPostToolUseDoesNotAutoQueueDraftWiki(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	raw := runHookCapture(t, `{
  "cwd": "`+repo+`",
  "tool_name": "Bash",
  "tool_input": {"command": "agent notes export"},
  "tool_response": "A durable observation says this might be reusable draft wiki material."
}`, func() error {
		return runHookPostToolUse([]string{"--json"})
	})
	if _, ok := raw["draft_wiki_queue"]; ok {
		t.Fatalf("PostToolUse must not auto-queue draft wiki material; main agent must judge and call project draft-wiki queue explicitly: %+v", raw)
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
