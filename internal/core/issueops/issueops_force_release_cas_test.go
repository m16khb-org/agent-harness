package issueops

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
)

func forceReleaseCASDigests(t *testing.T, stateRoot, id string) (string, string) {
	t.Helper()
	raw, err := readRawIssueOpsBytes(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	rawSHA, canonicalSHA, err := computeForceReleaseCASDigests(raw)
	if err != nil {
		t.Fatal(err)
	}
	return rawSHA, canonicalSHA
}

func TestComputeForceReleaseCASDigestsDoesNotHTMLEscapeCanonicalJSON(t *testing.T) {
	raw := []byte("{\"n\":7,\"html\":\"<&>\u2028\u2029\"}")
	_, canonicalSHA, err := computeForceReleaseCASDigests(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(`{"html":"<&>\u2028\u2029","n":7}`))
	if canonicalSHA != hex.EncodeToString(want[:]) {
		t.Fatalf("canonical digest uses a different JSON contract: got %s want %s", canonicalSHA, hex.EncodeToString(want[:]))
	}
}

func TestComputeForceReleaseCASDigestsPreservesJSONIntegersBeyondFloatPrecision(t *testing.T) {
	raw := []byte(`{"sequence":9007199254740993}`)
	_, canonicalSHA, err := computeForceReleaseCASDigests(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(raw)
	if canonicalSHA != hex.EncodeToString(want[:]) {
		t.Fatalf("canonical digest rounded a JSON integer: got %s want %s", canonicalSHA, hex.EncodeToString(want[:]))
	}
}

func startForceReleaseCASRecord(t *testing.T, stateRoot string) IssueOpsRecord {
	t.Helper()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{
		Repo: t.TempDir(), Branch: "77-force-release-cas",
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func forceReleaseCASStateRoot(t *testing.T) string {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	return IssueOpsStateRoot()
}

func forceReleaseCASRequest(t *testing.T, stateRoot, id string) ForceReleaseCASRequest {
	t.Helper()
	rawSHA, canonicalSHA := forceReleaseCASDigests(t, stateRoot, id)
	return ForceReleaseCASRequest{
		ExpectedRawSHA256:       rawSHA,
		ExpectedCanonicalSHA256: canonicalSHA,
	}
}

func TestForceReleaseIssueOpsCASReleasesExactUnboundRecord(t *testing.T) {
	stateRoot := forceReleaseCASStateRoot(t)
	record := startForceReleaseCASRecord(t, stateRoot)
	req := forceReleaseCASRequest(t, stateRoot, record.ID)

	result, err := ForceReleaseIssueOpsCAS(stateRoot, record.ID, "sealed reconciliation release", req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Record.Phase != IssueOpsPhaseDone || !result.BindingAbsenceVerified {
		t.Fatalf("unexpected CAS release result: %+v", result)
	}
	if result.BeforeRawSHA256 != req.ExpectedRawSHA256 || result.BeforeCanonicalSHA256 != req.ExpectedCanonicalSHA256 {
		t.Fatalf("before digests differ from request: %+v", result)
	}
	if result.AfterRawSHA256 == "" || result.AfterCanonicalSHA256 == "" || result.RepoBindingCountBefore != 0 || result.RepoBindingCountAfter != 0 {
		t.Fatalf("CAS proof is incomplete: %+v", result)
	}
}

func TestForceReleaseIssueOpsCASRejectsRawDriftWithoutMutation(t *testing.T) {
	stateRoot := forceReleaseCASStateRoot(t)
	record := startForceReleaseCASRecord(t, stateRoot)
	req := forceReleaseCASRequest(t, stateRoot, record.ID)
	req.ExpectedRawSHA256 = strings.Repeat("a", 64)
	before, err := readRawIssueOpsBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ForceReleaseIssueOpsCAS(stateRoot, record.ID, "sealed reconciliation release", req); err == nil || !strings.Contains(err.Error(), "raw SHA-256") {
		t.Fatalf("expected raw digest rejection, got %v", err)
	}
	after, err := readRawIssueOpsBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("digest rejection mutated the IssueOps record")
	}
}

func TestForceReleaseIssueOpsCASRejectsAndPreservesPrimaryBinding(t *testing.T) {
	stateRoot := forceReleaseCASStateRoot(t)
	record := startForceReleaseCASRecord(t, stateRoot)
	req := forceReleaseCASRequest(t, stateRoot, record.ID)
	if err := BindIssueOpsSession(record.Repo, record.ID, record.Branch, record.WorktreePath); err != nil {
		t.Fatal(err)
	}

	if _, err := ForceReleaseIssueOpsCAS(stateRoot, record.ID, "sealed reconciliation release", req); err == nil || !strings.Contains(err.Error(), "session binding") {
		t.Fatalf("expected primary binding rejection, got %v", err)
	}
	binding, err := ReadIssueOpsSession(record.Repo)
	if err != nil || binding.CycleID != record.ID {
		t.Fatalf("primary binding was not preserved: binding=%+v err=%v", binding, err)
	}
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || current.Phase == IssueOpsPhaseDone {
		t.Fatalf("binding rejection mutated the record: record=%+v err=%v", current, err)
	}
}

func TestForceReleaseIssueOpsCASRejectsAndPreservesScopedBinding(t *testing.T) {
	stateRoot := forceReleaseCASStateRoot(t)
	record := startForceReleaseCASRecord(t, stateRoot)
	req := forceReleaseCASRequest(t, stateRoot, record.ID)
	if err := BindScopedIssueOpsSession(record.Repo, record.ID, record.Branch, record.WorktreePath); err != nil {
		t.Fatal(err)
	}

	if _, err := ForceReleaseIssueOpsCAS(stateRoot, record.ID, "sealed reconciliation release", req); err == nil || !strings.Contains(err.Error(), "session binding") {
		t.Fatalf("expected scoped binding rejection, got %v", err)
	}
	binding, err := ReadScopedIssueOpsSession(record.Repo, record.ID)
	if err != nil || binding.CycleID != record.ID {
		t.Fatalf("scoped binding was not preserved: binding=%+v err=%v", binding, err)
	}
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || current.Phase == IssueOpsPhaseDone {
		t.Fatalf("binding rejection mutated the record: record=%+v err=%v", current, err)
	}
}

func TestForceReleaseIssueOpsCASSerializesConcurrentBindWithoutDeletingIt(t *testing.T) {
	for iteration := 0; iteration < 20; iteration++ {
		stateRoot := forceReleaseCASStateRoot(t)
		record := startForceReleaseCASRecord(t, stateRoot)
		req := forceReleaseCASRequest(t, stateRoot, record.ID)
		start := make(chan struct{})
		var releaseErr, bindErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, releaseErr = ForceReleaseIssueOpsCAS(stateRoot, record.ID, "sealed reconciliation release", req)
		}()
		go func() {
			defer wg.Done()
			<-start
			bindErr = BindIssueOpsSession(record.Repo, record.ID, record.Branch, record.WorktreePath)
		}()
		close(start)
		wg.Wait()
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		binding, err := ReadIssueOpsSession(record.Repo)
		if err != nil || binding.CycleID != record.ID {
			t.Fatalf("iteration %d deleted the racing binding: binding=%+v err=%v releaseErr=%v", iteration, binding, err, releaseErr)
		}
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if releaseErr == nil && current.Phase != IssueOpsPhaseDone {
			t.Fatalf("iteration %d successful release did not persist done", iteration)
		}
		if releaseErr != nil && current.Phase == IssueOpsPhaseDone {
			t.Fatalf("iteration %d rejected release still mutated record: %v", iteration, releaseErr)
		}
	}
}
