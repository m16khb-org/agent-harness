package issueops

import (
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/contract/issueops"
)

// TestStartIssueOpsLockIDMatchesAbsRecordID guards LK-01: the lock id that
// StartIssueOps acquires must equal the canonical record id that start.Start
// derives from the ABS-normalized repo (newIssueOpsID(abs(repo), branch)).
// Before the fix StartIssueOps hashed the RAW repo string, so a relative path
// (".") and its absolute equivalent took DIFFERENT locks while read-modify-
// writing the SAME record -> the lost-update TOCTOU the prior P0 fix closed.
// newIssueOpsID does no abs-normalization, so the raw and abs hashes differ for
// a relative path, which is exactly what this test pins down.
func TestStartIssueOpsLockIDMatchesAbsRecordID(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	branch := "12-demo"
	abs, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	// recordID is the id start.Start writes (it abs-normalizes the repo).
	recordID := newIssueOpsID(abs, branch)

	// Precondition: the raw (un-normalized) hash the buggy code used must be a
	// DIFFERENT id, otherwise this test could not detect the regression.
	if newIssueOpsID(".", branch) == recordID {
		t.Fatalf("test precondition: raw and abs ids must differ to expose LK-01")
	}

	if got := issueOpsStartLockID(".", branch); got != recordID {
		t.Fatalf("relative-path lock id must match abs-normalized record id, got %q want %q", got, recordID)
	}

	// The absolute path must resolve to the same lock id as the relative path.
	if got := issueOpsStartLockID(abs, branch); got != recordID {
		t.Fatalf("absolute-path lock id must match the relative-path lock id, got %q want %q", got, recordID)
	}
}

func issueOpsRecordExists(t *testing.T, stateRoot, id string) bool {
	t.Helper()
	_, err := ReadIssueOps(stateRoot, id)
	return err == nil
}

// TestStartIssueOpsRelativeThenAbsoluteShareOneRecordAndLock verifies the
// end-to-end effect of LK-01: starting with a relative repo and then with the
// equivalent absolute repo resolves to the SAME cycle record (resume, not a
// duplicate), and the lock that either call would take matches that record id.
func TestStartIssueOpsRelativeThenAbsoluteShareOneRecordAndLock(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	branch := "12-demo"
	first, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: ".", Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	second, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Fatalf("relative and absolute repo must resolve to one record, got %q and %q", first.ID, second.ID)
	}

	// The relative-path start must lock on the record id it mutates, not the
	// raw relative hash; same for the absolute-path start.
	if got := issueOpsStartLockID(".", branch); got != first.ID {
		t.Fatalf("relative-path lock id %q must match the record id %q it mutates", got, first.ID)
	}
	if got := issueOpsStartLockID(repo, branch); got != first.ID {
		t.Fatalf("absolute-path lock id %q must match the record id %q it mutates", got, first.ID)
	}

	// Behavioral proof that StartIssueOps writes under the abs-normalized record
	// id: the record exists under first.ID, while the raw relative-hash id the
	// buggy code would have used was never written.
	if !issueOpsRecordExists(t, stateRoot, first.ID) {
		t.Fatalf("StartIssueOps must persist under the abs-normalized record id %q", first.ID)
	}
	if issueOpsRecordExists(t, stateRoot, newIssueOpsID(".", branch)) {
		t.Fatalf("StartIssueOps must not persist under the raw relative hash")
	}
}
