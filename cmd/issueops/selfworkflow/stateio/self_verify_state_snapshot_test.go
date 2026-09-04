package stateio

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"issueops/cmd/issueops/selfworkflow/model"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/contract/failurecause"
)

func TestWriteSelfAugmentSnapshotRecordIsLockedAndAtomic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
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

	// The locked writer leaves the span-lock database but NO leftover temp file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var sawLockDB bool
	for _, e := range entries {
		name := e.Name()
		switch {
		case name == "issueops.lock.db":
			sawLockDB = true
		case strings.HasSuffix(name, ".tmp"):
			t.Fatalf("leftover temp file after write: %s", name)
		}
	}
	if !sawLockDB {
		t.Fatalf("span lock database missing (write not serialized)")
	}
}

func TestReadSelfAugmentStateSnapshotRejectsBadSchemaAndRetiredKinds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
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

	if err := WriteSelfAugmentSnapshotRecord(dir, "retired-kind", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write retired kind: %v", err)
	}
	if _, err := ReadSelfAugmentStateSnapshot("retired-kind"); err == nil || !strings.Contains(err.Error(), "contains kind") {
		t.Fatalf("expected retired kind rejection, got %v", err)
	}
}
func TestReadSelfAugmentStateSnapshotNormalizesLegacyFailureCause(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)

	for _, tc := range []struct {
		name    string
		content string
		cause   failurecause.Cause
	}{
		{
			name:    "success",
			content: `{"schema_version":1,"kind":"self_verification_summary","ok":true,"summary":{"total_steps":1,"passed_steps":1,"failed_steps":0}}`,
			cause:   failurecause.None,
		},
		{
			name:    "failure",
			content: `{"schema_version":1,"kind":"self_verification_summary","ok":false,"summary":{"total_steps":1,"failed_steps":1}}`,
			cause:   failurecause.Unknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := statestore.StateWrite("legacy-"+tc.name, tc.content); err != nil {
				t.Fatalf("write legacy state: %v", err)
			}

			snapshot, err := ReadSelfAugmentStateSnapshot("legacy-" + tc.name)
			if err != nil {
				t.Fatalf("read legacy state: %v", err)
			}
			if snapshot.Summary.FailureCause != tc.cause {
				t.Fatalf("failure cause = %q, want %q", snapshot.Summary.FailureCause, tc.cause)
			}
			if snapshot.Summary.FailureCauseEvidence == nil || len(snapshot.Summary.FailureCauseEvidence) != 0 {
				t.Fatalf("failure cause evidence = %#v, want empty slice", snapshot.Summary.FailureCauseEvidence)
			}
		})
	}
}

func TestReadSelfAugmentStateSnapshotRoundTripsFailureCause(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	evidence := []failurecause.Evidence{{
		Cause:  failurecause.Transport,
		Code:   "framing",
		Source: "mcp",
	}}
	want := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfVerificationSummaryKind,
		Summary: model.SelfAugmentSummary{
			TotalSteps:           1,
			FailedSteps:          1,
			FailureCause:         failurecause.Transport,
			FailureCauseReason:   "transport:framing",
			FailureCauseEvidence: evidence,
		},
	}

	if err := WriteSelfAugmentSnapshotRecord(dir, "cause-round-trip", want); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadSelfAugmentStateSnapshot("cause-round-trip")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", got.SchemaVersion)
	}
	if got.Summary.FailureCause != want.Summary.FailureCause || got.Summary.FailureCauseReason != want.Summary.FailureCauseReason {
		t.Fatalf("failure cause fields = %#v, want %#v", got.Summary, want.Summary)
	}
	if !reflect.DeepEqual(got.Summary.FailureCauseEvidence, want.Summary.FailureCauseEvidence) {
		t.Fatalf("failure cause evidence = %#v, want %#v", got.Summary.FailureCauseEvidence, want.Summary.FailureCauseEvidence)
	}
}
