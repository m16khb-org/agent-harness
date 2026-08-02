package executioncmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestExecutionResumeCLIParsesOnlyTheExactFlagSurface(t *testing.T) {
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{
		"resume", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "3",
		"--host", "codex", "--session-id", "session-resume",
		"--session-pid", fmt.Sprint(receipt.PID),
		"--session-started-at", receipt.StartedAt,
		"--session-executable", receipt.Executable,
		"--cwd", "/repo.worktrees/resume", "--confirm", "--json",
	}
	if err := Run(args, Deps{}); err == nil || !strings.Contains(err.Error(), "state root is unavailable") {
		t.Fatalf("exact resume flags did not reach execution routing: %v", err)
	}

	withSnapshot := append(append([]string(nil), args...), "--issue-snapshot-file", "/tmp/issue.json")
	if err := Run(withSnapshot, Deps{}); err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("resume accepted issue snapshot flag: %v", err)
	}
}

func TestExecutionResumeCLIInvokesInjectedHandler(t *testing.T) {
	stateRoot := t.TempDir()
	args := []string{
		"resume", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "3",
		"--host", "codex", "--session-id", "session-resume",
		"--session-pid", "42", "--session-started-at", "2026-07-31T00:00:00Z",
		"--session-executable", "/usr/local/bin/codex", "--cwd", "/repo.worktrees/resume", "--confirm", "--json",
	}
	calls := 0
	var output any
	err := Run(args, Deps{
		StateRoot: func() string { return stateRoot },
		Resume: func(_ context.Context, gotRoot string, request issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			calls++
			if gotRoot != stateRoot || request.ID != "io-aaaaaaaaaaaa" || request.ExpectedGeneration != 3 || request.CWD != "/repo.worktrees/resume" || !request.Confirm {
				t.Fatalf("resume handler request=%+v state_root=%q", request, gotRoot)
			}
			return issueops.ExecutionResumeResult{OK: true, ID: request.ID}, nil
		},
		PrintJSON: func(value any) error { output = value; return nil },
	})
	if err != nil || calls != 1 {
		t.Fatalf("resume CLI err=%v calls=%d", err, calls)
	}
	result, ok := output.(issueops.ExecutionResumeResult)
	if !ok || !result.OK || result.ID != "io-aaaaaaaaaaaa" {
		t.Fatalf("resume CLI output=%#v", output)
	}
}

func TestExecutionResumeUsageIsAdvertisedOnce(t *testing.T) {
	if got := strings.Count(Usage, "issueops execution resume"); got != 1 {
		t.Fatalf("resume usage count = %d", got)
	}
}
