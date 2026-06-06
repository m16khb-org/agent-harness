package hookfailure

import (
	"os"
	"path/filepath"
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

func TestListHookFailureEventsCoversEmptyInvalidAndLimit(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	empty, err := ListHookFailureEvents(0)
	if err != nil {
		t.Fatalf("ListHookFailureEvents(empty) error = %v", err)
	}
	if !empty.OK || len(empty.Events) != 0 {
		t.Fatalf("expected empty list from missing log: %+v", empty)
	}

	path := HookFailureLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir hook failure log dir: %v", err)
	}
	body := strings.Join([]string{
		`{"hook":"one","error":"first"}`,
		`not-json`,
		`{"hook":"two","error":"second"}`,
		`{"hook":"three","error":"third"}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write hook failure log: %v", err)
	}

	limited, err := ListHookFailureEvents(2)
	if err != nil {
		t.Fatalf("ListHookFailureEvents(limit) error = %v", err)
	}
	if !limited.OK || len(limited.Events) != 2 {
		t.Fatalf("expected two valid latest events: %+v", limited)
	}
	if limited.Events[0].Hook != "two" || limited.Events[1].Hook != "three" {
		t.Fatalf("unexpected event ordering after invalid JSON skip: %+v", limited.Events)
	}
}
