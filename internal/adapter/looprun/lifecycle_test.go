package looprun

import (
	loopruncontract "issueops/internal/contract/looprun"
	"strings"
	"testing"
)

func TestStartResumePreservesAttemptsAndRedactsGoal(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	req := loopruncontract.StartLoopRequest{
		Repo:        repo,
		Name:        "qa-loop",
		Goal:        "run tests with token=super-secret-value",
		VerifyArgv:  []string{"go", "test", "./..."},
		MaxAttempts: 2,
	}

	started, err := Start(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(started.Goal, "super-secret-value") || !strings.Contains(started.Goal, "<redacted>") {
		t.Fatalf("goal should redact secret-like assignments, got %q", started.Goal)
	}
	if len(started.VerifyArgv) != 3 || started.VerifyArgv[0] != "go" {
		t.Fatalf("verify argv should be stored exactly, got %+v", started.VerifyArgv)
	}
	if _, err := RecordAttempt(started.ID, loopruncontract.RecordAttemptRequest{Verdict: "fail", Evidence: []string{"go test failed"}}); err != nil {
		t.Fatal(err)
	}

	resumed, err := Start(req)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID != started.ID || resumed.Status != "active" || len(resumed.Attempts) != 1 {
		t.Fatalf("same repo+name should resume active loop with attempts: started=%+v resumed=%+v", started, resumed)
	}
	if resumed.Attempts[0].Seq != 1 || resumed.Attempts[0].Verdict != "fail" {
		t.Fatalf("resume should preserve attempt history, got %+v", resumed.Attempts)
	}
}

func TestRecordAttemptEvidenceRequired(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	loop := startLoopForTest(t, "evidence-required", 2)
	if _, err := RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{Verdict: "pass"}); err == nil || !strings.Contains(err.Error(), "evidence_required") {
		t.Fatalf("record attempt without evidence err=%v, want evidence_required", err)
	}
}

func TestRecordAttemptAutoExhaustRejectsFurtherAttempts(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	loop := startLoopForTest(t, "auto-exhaust", 1)
	exhausted, err := RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{Verdict: "fail", Evidence: []string{"first failure"}})
	if err != nil {
		t.Fatal(err)
	}
	if exhausted.Status != "exhausted" || len(exhausted.Attempts) != 1 {
		t.Fatalf("max fail should exhaust loop, got %+v", exhausted)
	}
	if _, err := RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{Verdict: "fail", Evidence: []string{"after exhausted"}}); err == nil || !strings.Contains(err.Error(), "loop_not_active") {
		t.Fatalf("attempt after exhausted err=%v, want loop_not_active", err)
	}
}

func TestStopSuccessRequiresPass(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	loop := startLoopForTest(t, "success-requires-pass", 2)
	if _, err := Stop(loop.ID, true, ""); err == nil || !strings.Contains(err.Error(), "loop_success_requires_pass") {
		t.Fatalf("success without pass err=%v, want loop_success_requires_pass", err)
	}
	if _, err := RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{Verdict: "fail", Evidence: []string{"still failing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Stop(loop.ID, true, ""); err == nil || !strings.Contains(err.Error(), "loop_success_requires_pass") {
		t.Fatalf("success after failing attempt err=%v, want loop_success_requires_pass", err)
	}
	if _, err := RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"all checks passed"}}); err != nil {
		t.Fatal(err)
	}
	stopped, err := Stop(loop.ID, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != "succeeded" {
		t.Fatalf("success stop should mark succeeded, got %+v", stopped)
	}
}

func TestStopReasonAndTerminalRestartRefusal(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	req := loopruncontract.StartLoopRequest{Repo: t.TempDir(), Name: "terminal", Goal: "prove stop reason", MaxAttempts: 2}
	loop, err := Start(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Stop(loop.ID, false, "short"); err == nil || !strings.Contains(err.Error(), "stop_reason_too_short") {
		t.Fatalf("short stop reason err=%v, want stop_reason_too_short", err)
	}
	stopped, err := Stop(loop.ID, false, "operator stopped this loop")
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != "stopped" || stopped.StopReason == "" {
		t.Fatalf("stop should record reason and terminal status, got %+v", stopped)
	}
	if _, err := Start(req); err == nil || !strings.Contains(err.Error(), "loop_terminal") {
		t.Fatalf("restart after terminal err=%v, want loop_terminal", err)
	}
}

func TestStatusReportsAttemptSummary(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	loop := startLoopForTest(t, "status", 3)
	if _, err := RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"green"}}); err != nil {
		t.Fatal(err)
	}
	status, err := Status(loop.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.OK || status.AttemptCount != 1 || status.LastVerdict != "pass" || status.Loop.ID != loop.ID {
		t.Fatalf("status summary mismatch: %+v", status)
	}
}

func startLoopForTest(t *testing.T, name string, maxAttempts int) loopruncontract.LoopRun {
	t.Helper()
	loop, err := Start(loopruncontract.StartLoopRequest{Repo: t.TempDir(), Name: name, Goal: "verify until done", MaxAttempts: maxAttempts})
	if err != nil {
		t.Fatal(err)
	}
	return loop
}
