package hookfailure

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/testsupport"
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
	return testsupport.CaptureStdout(t, func() error {
		fn()
		return nil
	})
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

// Q2 first-reading follow-up: help requests are not failures — 16 of the 38
// recorded "failures" were `flag: help requested` noise drowning the signal.
func TestRecordSkipsHelpRequestedErrors(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	Record([]string{"stop"}, nil, flag.ErrHelp)

	if _, err := os.Stat(filepath.Join(stateDir, "hook-failures.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("help-requested must not be recorded as a failure (stat err=%v)", err)
	}
}

func TestRunFailuresPruneAndMetricsJSON(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	oldLine := `{"timestamp":"2020-01-01T00:00:00Z","hook":"stop","error":"old"}` + "\n"
	if err := os.WriteFile(filepath.Join(stateDir, "hook-failures.jsonl"), []byte(oldLine), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"--prune", "1h", "--json"},
		{"prune", "--max-age", "1h", "--json"},
	} {
		out := captureStdout(t, func() {
			if err := Run(args); err != nil {
				t.Fatalf("Run(%v): %v", args, err)
			}
		})
		var obj map[string]any
		if err := json.Unmarshal([]byte(out), &obj); err != nil {
			t.Fatalf("prune output is not JSON: %q: %v", out, err)
		}
		if obj["ok"] != true {
			t.Fatalf("unexpected prune output: %+v", obj)
		}
	}

	out := captureStdout(t, func() {
		if err := RunMetrics([]string{"--json"}); err != nil {
			t.Fatalf("RunMetrics: %v", err)
		}
	})
	var metrics map[string]any
	if err := json.Unmarshal([]byte(out), &metrics); err != nil {
		t.Fatalf("metrics output is not JSON: %q: %v", out, err)
	}
	if metrics["ok"] != true {
		t.Fatalf("unexpected metrics output: %+v", metrics)
	}
}

func TestRecordFailureAndArgValue(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	stdin := []byte(`{"tool_name":"Bash","tool_input":{"command":"go test ./..."},"cwd":"/repo"}`)

	Record([]string{"pre-tool-use", "--host=codex", "--repo", "/repo"}, stdin, errors.New("denied"))

	if ArgValue([]string{"--repo", "/repo"}, "--repo") != "/repo" || ArgValue([]string{"--repo=/repo"}, "--repo") != "/repo" || ArgValue(nil, "--repo") != "" {
		t.Fatal("ArgValue did not parse expected flag forms")
	}
	out := captureStdout(t, func() {
		if err := Run([]string{"--limit", "1"}); err != nil {
			t.Fatalf("Run list: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("list output is not JSON: %q: %v", out, err)
	}
	events, _ := obj["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("expected recorded event, got %+v", obj)
	}
}
