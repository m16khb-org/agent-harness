package hookfailure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecordHookFailureEventConcurrentAppendsStayValid(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	const n = 50
	big := strings.Repeat("x", 480)                                            // near the 500B snippet cap
	longArgv := []string{strings.Repeat("a", 2000), strings.Repeat("b", 2000)} // push each line well past PIPE_BUF
	longCWD := strings.Repeat("/d", 1000)

	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = RecordHookFailureEvent(HookFailureEvent{
				Hook:           "stop",
				CommandSnippet: big,
				Error:          big,
				Argv:           longArgv,
				CWD:            longCWD,
			})
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(HookFailureLogPath())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d lines, got %d (interleaved/lost writes)", n, len(lines))
	}
	for i, ln := range lines {
		var ev HookFailureEvent
		if err := json.Unmarshal([]byte(ln), &ev); err != nil {
			t.Fatalf("line %d is not valid JSON (torn concurrent append): %v", i, err)
		}
	}
}

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

func TestPruneHookFailureLogRemovesOldEntries(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	path := HookFailureLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir hook failure log dir: %v", err)
	}

	oldTime := time.Now().UTC().Add(-800 * time.Hour).Format(time.RFC3339Nano)
	newTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339Nano)

	body := strings.Join([]string{
		`{"timestamp":"` + oldTime + `","hook":"old-one","error":"expired"}`,
		`{"timestamp":"` + newTime + `","hook":"recent","error":"kept"}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write hook failure log: %v", err)
	}

	result, err := PruneHookFailureLog(720 * time.Hour)
	if err != nil {
		t.Fatalf("PruneHookFailureLog error = %v", err)
	}
	if !result.OK {
		t.Fatalf("expected OK pruned result: %+v", result)
	}
	if result.Pruned != 1 {
		t.Fatalf("expected 1 pruned entry, got %d", result.Pruned)
	}
	if result.Kept != 1 {
		t.Fatalf("expected 1 kept entry, got %d", result.Kept)
	}

	// Verify the file now contains only the recent entry
	limited, err := ListHookFailureEvents(0)
	if err != nil {
		t.Fatalf("ListHookFailureEvents after prune error = %v", err)
	}
	if len(limited.Events) != 1 {
		t.Fatalf("expected 1 event after prune, got %d: %+v", len(limited.Events), limited.Events)
	}
	if limited.Events[0].Hook != "recent" {
		t.Fatalf("expected 'recent' hook, got %q", limited.Events[0].Hook)
	}
}

func TestPruneHookFailureLogEmptyFile(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	// No file exists yet
	result, err := PruneHookFailureLog(720 * time.Hour)
	if err != nil {
		t.Fatalf("PruneHookFailureLog on missing file error = %v", err)
	}
	if !result.OK {
		t.Fatalf("expected OK for missing file: %+v", result)
	}
	if result.Pruned != 0 || result.Kept != 0 {
		t.Fatalf("expected zero operations on missing file: %+v", result)
	}
}
