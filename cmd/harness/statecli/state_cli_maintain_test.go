package statecli

import (
	"encoding/json"
	"strings"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	statestore "agent-harness/internal/adapter/outbound/state"
	issueopscontract "agent-harness/internal/contract/issueops"
	statecontract "agent-harness/internal/contract/state"
)

func TestRunStateMaintainReportsRoots(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	// Materialize the root store and the IssueOps v1 store; worker and loop stay
	// absent and must be reported as skipped, not created.
	if _, err := statestore.StateWrite("maintain-smoke", "content"); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            issueopscore.NewIssueOpsID("/repo/maintain", "1-maintain"),
		Repo:          "/repo/maintain",
		Phase:         issueopscore.IssueOpsPhaseProblem,
	}); err != nil {
		t.Fatalf("seed issueops: %v", err)
	}

	out := captureStatusVerifyStdout(t, func() error {
		return runState(testDependencies(), []string{"maintain", "--json"})
	})
	var result statecontract.StateMaintainResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode maintain JSON: %v\n%s", err, out)
	}
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
	if len(result.Roots) != 2 {
		t.Fatalf("expected 2 maintained roots (state, issueops), got %+v", result)
	}
	if len(result.Skipped) != 2 {
		t.Fatalf("expected 2 skipped roots (worker, loop), got %+v", result)
	}
	for _, root := range result.Skipped {
		if !strings.HasSuffix(root, "/worker") && !strings.HasSuffix(root, "/loop") {
			t.Fatalf("unexpected skipped root %q in %+v", root, result)
		}
	}

	text := captureStatusVerifyStdout(t, func() error {
		return runState(testDependencies(), []string{"maintain"})
	})
	if !strings.Contains(text, "maintained 2 store roots") {
		t.Fatalf("unexpected maintain text:\n%s", text)
	}
}
