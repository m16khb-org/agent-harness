package issueops

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestIncrementIssueOpsSourceMisdirectAccumulates(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "84-misdirect"})
	if err != nil {
		t.Fatal(err)
	}
	for want := 1; want <= 2; want++ {
		count, err := IncrementIssueOpsSourceMisdirect(stateRoot, record.ID)
		if err != nil || count != want {
			t.Fatalf("counter must accumulate to %d: %v %d", want, err, count)
		}
	}
	ready := IssueOpsStrictPRReadinessWithState(stateRoot, mustReadForMisdirectTest(t, stateRoot, record.ID))
	found := false
	for _, warning := range ready.Warnings {
		if warning == "source_misdirect_warnings:2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("strict readiness must surface the misdirect warning key: %+v", ready.Warnings)
	}
}

func mustReadForMisdirectTest(t *testing.T, stateRoot, id string) issueops.IssueOpsRecord {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	return record
}
