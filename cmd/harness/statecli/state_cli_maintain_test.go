package statecli

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunStateMaintainReportsRoots(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	// Materialize the root store and the issueops store; workpool, worker, and
	// loop stay absent and must be reported as skipped, not created.
	if _, err := core.StateWrite("maintain-smoke", "content"); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), core.IssueOpsRecord{
		ID:    core.NewIssueOpsID("/repo/maintain", "1-maintain"),
		Repo:  "/repo/maintain",
		Phase: core.IssueOpsPhaseProblem,
	}); err != nil {
		t.Fatalf("seed issueops: %v", err)
	}

	out := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"maintain", "--json"})
	})
	var result core.StateMaintainResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode maintain JSON: %v\n%s", err, out)
	}
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
	if len(result.Roots) != 2 {
		t.Fatalf("expected 2 maintained roots (state, issueops), got %+v", result)
	}
	if len(result.Skipped) != 3 {
		t.Fatalf("expected 3 skipped roots (workpool, worker, loop), got %+v", result)
	}

	text := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"maintain"})
	})
	if !strings.Contains(text, "maintained 2 store roots") {
		t.Fatalf("unexpected maintain text:\n%s", text)
	}
}
