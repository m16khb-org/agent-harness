package stateio

import (
	"os"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
)

func TestWriteSelfAugmentSnapshotRecordIsLockedAndAtomic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := model.SelfAugmentSummary{TotalRuns: 2, TotalSteps: 5, PassedSteps: 5}

	if err := WriteSelfAugmentSnapshotRecord(dir, "snap", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfVerificationSummaryKind,
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Round-trips through the normal reader (record path unchanged).
	got, err := ReadSelfAugmentStateSnapshot("snap")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Summary.TotalSteps != 5 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	// The atomic writer leaves the advisory lock sidecar but NO leftover temp file,
	// and the lock file is not mistaken for the record.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var sawRecord, sawLock bool
	for _, e := range entries {
		name := e.Name()
		switch {
		case name == "snap.json":
			sawRecord = true
		case name == "snap.state-lock":
			sawLock = true
		case strings.HasSuffix(name, ".tmp"):
			t.Fatalf("leftover temp file after atomic write: %s", name)
		}
	}
	if !sawRecord {
		t.Fatalf("record file snap.json missing")
	}
	if !sawLock {
		t.Fatalf("advisory lock sidecar snap.state-lock missing (write not serialized)")
	}
}

func TestReadSelfAugmentStateSnapshotRejectsBadKindAndSchemaAndAcceptsLegacyKind(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := model.SelfAugmentSummary{TotalRuns: 1, TotalSteps: 1, PassedSteps: 1}

	if err := WriteSelfAugmentSnapshotRecord(dir, "bad-kind", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "other",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write bad kind: %v", err)
	}
	if _, err := ReadSelfAugmentStateSnapshot("bad-kind"); err == nil || !strings.Contains(err.Error(), "contains kind") {
		t.Fatalf("expected bad kind error, got %v", err)
	}

	if err := WriteSelfAugmentSnapshotRecord(dir, "bad-schema", SelfAugmentStateSnapshot{
		SchemaVersion: 2,
		Kind:          model.SelfVerificationSummaryKind,
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write bad schema: %v", err)
	}
	if _, err := ReadSelfAugmentStateSnapshot("bad-schema"); err == nil || !strings.Contains(err.Error(), "unsupported self-verification summary schema") {
		t.Fatalf("expected bad schema error, got %v", err)
	}

	if err := WriteSelfAugmentSnapshotRecord(dir, "legacy", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.LegacySelfAugmentSummaryKind,
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write legacy kind: %v", err)
	}
	if snapshot, err := ReadSelfAugmentStateSnapshot("legacy"); err != nil || snapshot.Kind != model.LegacySelfAugmentSummaryKind {
		t.Fatalf("expected legacy kind to read, snapshot=%+v err=%v", snapshot, err)
	}
}
