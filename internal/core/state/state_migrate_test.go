package state

import (
	"encoding/json"
	"testing"
)

func TestStateMigrateDryRunAndConfirm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	legacy := StateRecord{
		Key:       "legacy",
		Content:   "legacy content",
		UpdatedAt: "2000-01-01T00:00:00Z",
		Bytes:     len([]byte("legacy content")),
	}
	b, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeRawStateRow(t, dir, "legacy", string(b)+"\n")
	if _, err := StateWrite("current", "current content"); err != nil {
		t.Fatalf("StateWrite current: %v", err)
	}

	doctorBefore, err := StateDoctor()
	if err != nil {
		t.Fatalf("StateDoctor before: %v", err)
	}
	if doctorBefore.Healthy || !stateDoctorHasIssue(doctorBefore.Issues, "legacy_schema") {
		t.Fatalf("doctor did not report legacy schema: %+v", doctorBefore)
	}

	dry, err := StateMigrate(false)
	if err != nil {
		t.Fatalf("StateMigrate dry-run: %v", err)
	}
	if !dry.OK || !dry.DryRun || dry.Confirm || !containsString(dry.CandidateKeys, "legacy") || len(dry.MigratedKeys) != 0 || !containsString(dry.SkippedKeys, "current") {
		t.Fatalf("unexpected dry-run migrate result: %+v", dry)
	}
	readLegacy, err := StateRead("legacy")
	if err != nil {
		t.Fatalf("StateRead legacy after dry-run: %v", err)
	}
	if readLegacy.Record.SchemaVersion != 0 {
		t.Fatalf("dry-run changed schema version to %d", readLegacy.Record.SchemaVersion)
	}

	confirmed, err := StateMigrate(true)
	if err != nil {
		t.Fatalf("StateMigrate confirm: %v", err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Confirm || !containsString(confirmed.CandidateKeys, "legacy") || !containsString(confirmed.MigratedKeys, "legacy") {
		t.Fatalf("unexpected confirmed migrate result: %+v", confirmed)
	}
	migrated, err := StateRead("legacy")
	if err != nil {
		t.Fatalf("StateRead legacy after migrate: %v", err)
	}
	if migrated.Record.SchemaVersion != StateCurrentSchemaVersion || migrated.Record.Content != legacy.Content || migrated.Record.UpdatedAt != legacy.UpdatedAt {
		t.Fatalf("unexpected migrated record: %+v", migrated.Record)
	}
	doctorAfter, err := StateDoctor()
	if err != nil {
		t.Fatalf("StateDoctor after: %v", err)
	}
	if !doctorAfter.Healthy {
		t.Fatalf("doctor should be healthy after migration: %+v", doctorAfter)
	}
}

func TestStateReadRejectsUnsupportedSchema(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	record := StateRecord{
		SchemaVersion: StateCurrentSchemaVersion + 1,
		Key:           "future",
		Content:       "future",
		UpdatedAt:     "2000-01-01T00:00:00Z",
		Bytes:         len([]byte("future")),
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeRawStateRow(t, dir, "future", string(b)+"\n")
	if _, err := StateRead("future"); err == nil {
		t.Fatalf("StateRead accepted unsupported schema")
	}
	doctor, err := StateDoctor()
	if err != nil {
		t.Fatalf("StateDoctor: %v", err)
	}
	if !stateDoctorHasIssue(doctor.Issues, "unsupported_schema") {
		t.Fatalf("doctor did not report unsupported schema: %+v", doctor)
	}
}
