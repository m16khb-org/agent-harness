package hookcli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
