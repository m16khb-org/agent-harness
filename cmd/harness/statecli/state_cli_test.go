package statecli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	statecontract "agent-harness/internal/contract/state"

	"agent-harness/internal/core"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/testsupport"
)

func TestRunStateRoutesUsageAndTextRoundtrip(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	stderr := captureStateCLIStderr(t, func() error {
		return runState(nil)
	})
	if !strings.Contains(stderr, "agent-harness state write") {
		t.Fatalf("state usage missing write command:\n%s", stderr)
	}
	stderr = captureStateCLIStderr(t, func() error {
		return runState([]string{"unknown"})
	})
	if !strings.Contains(stderr, "agent-harness state migrate") {
		t.Fatalf("state usage missing migrate command:\n%s", stderr)
	}

	input := filepath.Join(t.TempDir(), "state.txt")
	if err := os.WriteFile(input, []byte("state from file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeOut := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"write", "--key", "text-key", "--input", input})
	})
	if !strings.Contains(writeOut, `state "text-key" written`) || !strings.Contains(writeOut, "16 bytes") {
		t.Fatalf("unexpected state write text output:\n%s", writeOut)
	}
	readOut := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"read", "text-key"})
	})
	if readOut != "state from file\n" {
		t.Fatalf("unexpected state read text output %q", readOut)
	}
	listOut := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"list"})
	})
	if !strings.Contains(listOut, "text-key\n") {
		t.Fatalf("unexpected state list text output:\n%s", listOut)
	}
}

func TestRunStateWriteReadAndPruneErrorsStaySurfaced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	for _, args := range [][]string{
		{"write", "--key", "missing-source"},
		{"write", "--key", "too-many", "--value", "one", "--input", filepath.Join(t.TempDir(), "missing.txt")},
	} {
		if err := runState(args); err == nil || !strings.Contains(err.Error(), "provide exactly one content source") {
			t.Fatalf("runState(%v) error=%v, want content source error", args, err)
		}
	}
	if err := runState([]string{"write", "--key", "missing-input", "--input", filepath.Join(t.TempDir(), "missing.txt")}); err == nil {
		t.Fatal("expected missing input file error")
	}
	if err := runState([]string{"read", "--key", "missing"}); err == nil {
		t.Fatal("expected missing read key error")
	}
	if err := runState([]string{"prune", "--max-age", "0s"}); err == nil {
		t.Fatal("expected invalid prune max-age error")
	}
}

func TestRunStatePruneDoctorAndMigrateTextBranches(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if _, err := core.StateWrite("old", "old content"); err != nil {
		t.Fatalf("write old state: %v", err)
	}
	old, err := core.StateRead("old")
	if err != nil {
		t.Fatalf("read old state: %v", err)
	}
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	writeStateCLIRecord(t, stateDir, "old", old.Record)

	dryPrune := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"prune", "--max-age", "1h"})
	})
	if !strings.Contains(dryPrune, "would prune 1 state records") || !strings.Contains(dryPrune, "old\n") {
		t.Fatalf("unexpected dry-run prune text:\n%s", dryPrune)
	}
	confirmedPrune := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"prune", "--max-age", "1h", "--confirm"})
	})
	if !strings.Contains(confirmedPrune, "pruned 1 state records") || !strings.Contains(confirmedPrune, "old\n") {
		t.Fatalf("unexpected confirmed prune text:\n%s", confirmedPrune)
	}

	writeRawStateCLIRow(t, stateDir, "corrupt", "{bad json\n")
	doctorOut := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"doctor"})
	})
	if !strings.Contains(doctorOut, "state doctor found 1 issues") || !strings.Contains(doctorOut, "error invalid_json") {
		t.Fatalf("unexpected doctor text:\n%s", doctorOut)
	}
	doctor, err := core.StateDoctor()
	if err != nil {
		t.Fatalf("state doctor: %v", err)
	}
	if !stateDoctorHasIssueCode(doctor.Issues, "invalid_json") || stateDoctorHasIssueCode(doctor.Issues, "missing") {
		t.Fatalf("stateDoctorHasIssueCode mismatch for issues: %#v", doctor.Issues)
	}

	migrateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", migrateDir)
	legacy := statecontract.RecordEnvelope{Key: "legacy", Content: "legacy content", UpdatedAt: "2000-01-01T00:00:00Z", Bytes: len([]byte("legacy content"))}
	writeStateCLIRecord(t, migrateDir, "legacy", legacy)
	dryMigrate := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"migrate"})
	})
	if !strings.Contains(dryMigrate, "would migrate 1 state records") || !strings.Contains(dryMigrate, "legacy\n") {
		t.Fatalf("unexpected dry-run migrate text:\n%s", dryMigrate)
	}
	confirmedMigrate := captureStatusVerifyStdout(t, func() error {
		return runState([]string{"migrate", "--confirm"})
	})
	if !strings.Contains(confirmedMigrate, "migrated 1 state records") || !strings.Contains(confirmedMigrate, "legacy\n") {
		t.Fatalf("unexpected confirmed migrate text:\n%s", confirmedMigrate)
	}
}

func writeStateCLIRecord(t *testing.T, dir, key string, record statecontract.RecordEnvelope) {
	t.Helper()
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeRawStateCLIRow(t, dir, key, string(b)+"\n")
}

func writeRawStateCLIRow(t *testing.T, dir, key, raw string) {
	t.Helper()
	db, err := sqlstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("state", key, []byte(raw)); err != nil {
		t.Fatal(err)
	}
}

func captureStateCLIStderr(t *testing.T, fn func() error) string {
	t.Helper()
	out, err := testsupport.CaptureStderrAndError(t, fn)
	if err == nil {
		t.Fatal("expected captured call to fail")
	}
	return out
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func stateDoctorHasIssueCode(issues []core.StateDoctorIssue, want string) bool {
	for _, issue := range issues {
		if issue.Code == want {
			return true
		}
	}
	return false
}
