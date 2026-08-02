package issueops

import (
	"testing"

	"agent-harness/internal/contract/issueops"
)

// Status display fills in a derived phase ledger when none was stamped, so old
// records still show phase progress. It is read-only (does not persist).
func TestIssueOpsStatusDerivesLedgerWhenAbsent(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	rec, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-status"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, rec.ID)

	status, err := IssueOpsStatus(stateRoot, rec.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(status.PhaseLedger) == 0 {
		t.Fatalf("status should derive a phase ledger for a record without one")
	}
	problem, ok := status.PhaseLedger[IssueOpsPhaseProblem]
	if !ok || len(problem.Missing) != 0 {
		t.Fatalf("status ledger should mark problem complete (intent present): %#v", problem)
	}
	// read-only: the persisted record still has no stored ledger
	persisted, err := ReadIssueOps(stateRoot, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.PhaseLedger) != 0 {
		t.Fatalf("status derivation must not persist the ledger: %#v", persisted.PhaseLedger)
	}
}
