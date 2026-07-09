package core

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/core/looprun"
)

func TestIssueOpsStrictPRReadinessBlocksActiveLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForPoolGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "active-loop", 3)
	ready := IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsCorePoolGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("active loop should block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessIgnoresOtherRepoLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForPoolGateTest(t)

	startCoreLoopGateLoop(t, filepath.Join(t.TempDir(), "other-repo"), "other-loop", 3)
	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsCorePoolGateString(ready.Missing, "loop_incomplete:") {
		t.Fatalf("other repo loop should not block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessBlocksExhaustedLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForPoolGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "exhausted-loop", 1)
	if _, err := looprun.RecordAttempt(loop.ID, looprun.RecordAttemptRequest{
		Verdict:  "fail",
		Evidence: []string{"focused verification failed"},
	}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	ready := IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsCorePoolGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("exhausted loop should block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessClearsAfterLoopStop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForPoolGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "stopped-loop", 3)
	if _, err := looprun.Stop(loop.ID, false, "operator stopped loop after explicit handoff"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsCorePoolGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("stopped loop should clear strict readiness, got %+v", ready)
	}

	successLoop := startCoreLoopGateLoop(t, record.Repo, "succeeded-loop", 3)
	if _, err := looprun.RecordAttempt(successLoop.ID, looprun.RecordAttemptRequest{
		Verdict:  "pass",
		Evidence: []string{"focused verification passed"},
	}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	if _, err := looprun.Stop(successLoop.ID, true, ""); err != nil {
		t.Fatalf("Stop success: %v", err)
	}
	ready = IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsCorePoolGateString(ready.Missing, "loop_incomplete:"+successLoop.ID) {
		t.Fatalf("succeeded loop should clear strict readiness, got %+v", ready)
	}
}

func startCoreLoopGateLoop(t *testing.T, repo, name string, maxAttempts int) looprun.LoopRun {
	t.Helper()
	loop, err := looprun.Start(looprun.StartLoopRequest{
		Repo:        repo,
		Name:        name,
		Goal:        "verify strict loop gate behavior",
		MaxAttempts: maxAttempts,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return loop
}
