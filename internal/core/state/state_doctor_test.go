package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateDoctorDetectsCorruptRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	if _, err := StateWrite("good", "good content"); err != nil {
		t.Fatalf("StateWrite good: %v", err)
	}
	writeRawStateRow(t, dir, "corrupt", "{not json\n")
	writeRawStateRow(t, dir, "badbytes", `{"key":"badbytes","content":"abc","updated_at":"2000-01-01T00:00:00Z","bytes":999}`+"\n")
	writeRawStateRow(t, dir, "badtime", `{"key":"badtime","content":"abc","updated_at":"not-a-time","bytes":3}`+"\n")

	result, err := StateDoctor()
	if err != nil {
		t.Fatalf("StateDoctor: %v", err)
	}
	if !result.OK || result.Healthy {
		t.Fatalf("unexpected doctor health: %+v", result)
	}
	if result.Checked != 4 {
		t.Fatalf("Checked=%d want 4", result.Checked)
	}
	if !containsString(result.ValidKeys, "good") {
		t.Fatalf("good key missing from valid keys: %+v", result)
	}
	for _, code := range []string{"invalid_json", "byte_count_mismatch", "invalid_timestamp"} {
		if !stateDoctorHasIssue(result.Issues, code) {
			t.Fatalf("missing issue %s: %+v", code, result.Issues)
		}
	}
}

func TestStateDoctorEmptyDirIsHealthy(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	result, err := StateDoctor()
	if err != nil {
		t.Fatalf("StateDoctor: %v", err)
	}
	if !result.OK || !result.Healthy || result.Checked != 0 || len(result.Issues) != 0 {
		t.Fatalf("unexpected empty doctor result: %+v", result)
	}
}

func TestStateDoctorAllowsHarnessOwnedAuxiliaryState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	if _, err := StateWrite("good", "good content"); err != nil {
		t.Fatalf("StateWrite good: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hook-failures.jsonl"), []byte(`{"hook":"pre-tool-use","error":"failed"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.state-lock"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "issueops-benchmarks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "audit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unknown.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "unknown-dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := StateDoctor()
	if err != nil {
		t.Fatalf("StateDoctor: %v", err)
	}
	if result.Healthy {
		t.Fatalf("unknown auxiliary state should still keep doctor unhealthy: %+v", result)
	}
	for _, issue := range result.Issues {
		if strings.Contains(issue.Path, "hook-failures.jsonl") || strings.Contains(issue.Path, ".state-lock") || strings.Contains(issue.Path, "issueops-benchmarks") || strings.Contains(issue.Path, "audit") {
			t.Fatalf("harness-owned auxiliary state should not warn: %+v", result.Issues)
		}
	}
	for _, code := range []string{"unexpected_file", "unexpected_directory"} {
		if !stateDoctorHasIssue(result.Issues, code) {
			t.Fatalf("missing issue %s for unknown auxiliary state: %+v", code, result.Issues)
		}
	}
}
