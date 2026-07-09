package loopcli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/looprun"
)

func TestRunLoopLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	startOut := captureLoopStdout(t, func() error {
		return Run([]string{"start", "--repo", repo, "--name", "cli loop", "--goal", "tests pass", "--max-attempts", "2", "--json", "--", "go", "test", "./..."})
	})
	var loop looprun.LoopRun
	unmarshalLoopJSON(t, startOut, &loop)
	if !loop.OK || loop.Status != "active" || len(loop.VerifyArgv) != 3 || loop.VerifyArgv[0] != "go" {
		t.Fatalf("unexpected start loop: %+v", loop)
	}

	failOut := captureLoopStdout(t, func() error {
		return Run([]string{"record-attempt", "--id", loop.ID, "--verdict", "fail", "--evidence", "one failure", "--json"})
	})
	unmarshalLoopJSON(t, failOut, &loop)
	if len(loop.Attempts) != 1 || loop.Attempts[0].Verdict != "fail" {
		t.Fatalf("unexpected fail attempt: %+v", loop)
	}

	passOut := captureLoopStdout(t, func() error {
		return Run([]string{"record-attempt", "--id", loop.ID, "--verdict", "pass", "--evidence", "all green", "--json"})
	})
	unmarshalLoopJSON(t, passOut, &loop)
	if len(loop.Attempts) != 2 || loop.Attempts[1].Verdict != "pass" {
		t.Fatalf("unexpected pass attempt: %+v", loop)
	}

	stopOut := captureLoopStdout(t, func() error {
		return Run([]string{"stop", "--id", loop.ID, "--success", "--json"})
	})
	unmarshalLoopJSON(t, stopOut, &loop)
	if loop.Status != "succeeded" {
		t.Fatalf("unexpected stop loop: %+v", loop)
	}

	statusOut := captureLoopStdout(t, func() error {
		return Run([]string{"status", "--id", loop.ID, "--json"})
	})
	var status looprun.StatusResult
	unmarshalLoopJSON(t, statusOut, &status)
	if !status.OK || status.AttemptCount != 2 || status.LastVerdict != "pass" || status.Loop.Status != "succeeded" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRunLoopExhaustionFailures(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	startOut := captureLoopStdout(t, func() error {
		return Run([]string{"start", "--repo", repo, "--name", "exhaust", "--goal", "fail closed", "--max-attempts", "1", "--json"})
	})
	var loop looprun.LoopRun
	unmarshalLoopJSON(t, startOut, &loop)
	recordOut := captureLoopStdout(t, func() error {
		return Run([]string{"record-attempt", "--id", loop.ID, "--verdict", "fail", "--evidence", "failed once", "--json"})
	})
	unmarshalLoopJSON(t, recordOut, &loop)
	if loop.Status != "exhausted" {
		t.Fatalf("single fail should exhaust, got %+v", loop)
	}
	if err := Run([]string{"record-attempt", "--id", loop.ID, "--verdict", "fail", "--evidence", "after exhausted", "--json"}); err == nil {
		t.Fatal("record-attempt after exhausted should fail")
	}
	if err := Run([]string{"stop", "--id", loop.ID, "--success", "--json"}); err == nil {
		t.Fatal("success stop after failed exhausted loop should fail")
	}
}

func captureLoopStdout(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}

func unmarshalLoopJSON(t *testing.T, out string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), target); err != nil {
		t.Fatalf("unmarshal loop JSON: %v\n%s", err, out)
	}
}
