package hookfailure

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunFailuresJSON(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if err := os.WriteFile(filepath.Join(stateDir, "hook-failures.jsonl"), []byte(`{"timestamp":"2026-06-02T00:00:00Z","hook":"pre-tool-use","error":"failed"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := Run([]string{"--json"}); err != nil {
			t.Fatalf("run hook failures: %v", err)
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRunFailuresStatsJSON(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	lines := `{"timestamp":"2026-06-02T00:00:00Z","hook":"pre-tool-use","error":"failed"}` + "\n" +
		`{"timestamp":"2026-06-02T00:00:01Z","hook":"stop","error":"failed"}` + "\n"
	if err := os.WriteFile(filepath.Join(stateDir, "hook-failures.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := Run([]string{"stats", "--json"}); err != nil {
			t.Fatalf("run hook failures stats: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("stats output is not JSON: %q: %v", out, err)
	}
	if obj["ok"] != true || obj["total"] != float64(2) {
		t.Fatalf("unexpected stats output: %+v", obj)
	}
	byHook, _ := obj["by_hook"].(map[string]any)
	if byHook["stop"] != float64(1) || byHook["pre-tool-use"] != float64(1) {
		t.Fatalf("unexpected by_hook: %+v", byHook)
	}
}
