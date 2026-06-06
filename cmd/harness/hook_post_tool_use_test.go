package main

import (
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
