package hookcli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHookPostToolUseDoesNotAutoQueueDraftWiki(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	raw := runHookCapture(t, `{
  "cwd": "`+repo+`",
  "tool_name": "Bash",
  "tool_input": {"command": "claude-mem export observations"},
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
