package core

import (
	"os"
	"strings"
	"testing"
)

func TestRecordHookFailureEventWritesRedactedJSONL(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	result, err := RecordHookFailureEvent(HookFailureEvent{
		Hook:           "pre-tool-use",
		Host:           "codex",
		Repo:           "/repo",
		CWD:            "/repo",
		Tool:           "Bash",
		Argv:           []string{"pre-tool-use", "--host", "codex"},
		CommandSnippet: "echo TOKEN=secret-value && rg hook cmd",
		Error:          "example TOKEN=secret-value failure",
	})

	if err != nil {
		t.Fatalf("RecordHookFailureEvent() error = %v", err)
	}
	if !result.OK {
		t.Fatalf("RecordHookFailureEvent() OK = false")
	}
	body, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read JSONL: %v", err)
	}
	if strings.Contains(string(body), "secret-value") {
		t.Fatalf("JSONL leaked secret: %s", string(body))
	}
	if !strings.Contains(string(body), `"hook":"pre-tool-use"`) {
		t.Fatalf("JSONL missing hook: %s", string(body))
	}
}
