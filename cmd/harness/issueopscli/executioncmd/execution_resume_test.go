package executioncmd

import (
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

func TestExecutionResumeUsageIsAdvertisedOnce(t *testing.T) {
	if got := strings.Count(Usage, "issueops execution resume"); got != 1 {
		t.Fatalf("resume usage count = %d", got)
	}
}
