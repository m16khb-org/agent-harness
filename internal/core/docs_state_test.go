package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDocsIndexIncludesAgentDocs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	index := DocsIndex(root, "test")
	if !index.OK {
		t.Fatalf("DocsIndex ok=false: %+v", index)
	}
	for _, want := range []string{"AGENTS.md", "CLAUDE.md", ".agent-harness/COMMIT_POLICY.md", ".agent-harness/OPERATIONS.md"} {
		if !docIndexContains(index.Docs, want) {
			t.Fatalf("DocsIndex missing %s: %+v", want, index.Docs)
		}
	}
	for _, doc := range index.Docs {
		if doc.Title == "" {
			t.Fatalf("doc %s has empty title", doc.RelPath)
		}
	}
}

func TestDocsIndexExcludesDraftWiki(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# Rules\n")
	mustWrite(t, filepath.Join(root, ProjectDocsDir, "CAUTIONS.md"), "# Cautions\n")
	mustWrite(t, filepath.Join(root, DraftWikiDir, "draft", "candidate.md"), "# Draft candidate\n")

	index := DocsIndex(root, "test")
	if !docIndexContains(index.Docs, "AGENTS.md") {
		t.Fatalf("DocsIndex missing AGENTS.md: %+v", index.Docs)
	}
	if !docIndexContains(index.Docs, ".agent-harness/CAUTIONS.md") {
		t.Fatalf("DocsIndex missing CAUTIONS.md: %+v", index.Docs)
	}
	if docIndexContains(index.Docs, ".agent-harness/draft-wiki/draft/candidate.md") {
		t.Fatalf("DocsIndex included draft-wiki candidate: %+v", index.Docs)
	}
}

func TestStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)

	content := "seed=42\nLore: state roundtrip\n"
	written, err := StateWrite("checkpoint-1", content)
	if err != nil {
		t.Fatalf("StateWrite: %v", err)
	}
	if !written.OK {
		t.Fatalf("StateWrite ok=false: %+v", written)
	}
	if written.StateDir != dir {
		t.Fatalf("StateDir=%q want %q", written.StateDir, dir)
	}
	if written.Path != filepath.Join(dir, "checkpoint-1.json") {
		t.Fatalf("Path=%q", written.Path)
	}
	if written.Record.Bytes != len([]byte(content)) {
		t.Fatalf("Bytes=%d want %d", written.Record.Bytes, len([]byte(content)))
	}
	if written.Record.SchemaVersion != StateCurrentSchemaVersion {
		t.Fatalf("SchemaVersion=%d want %d", written.Record.SchemaVersion, StateCurrentSchemaVersion)
	}

	read, err := StateRead("checkpoint-1")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	if read.Record.Content != content {
		t.Fatalf("content=%q want %q", read.Record.Content, content)
	}

	listed, err := StateList()
	if err != nil {
		t.Fatalf("StateList: %v", err)
	}
	if len(listed.Keys) != 1 || listed.Keys[0] != "checkpoint-1" {
		t.Fatalf("Keys=%v", listed.Keys)
	}
	if len(listed.Records) != 1 || listed.Records[0].Bytes != len([]byte(content)) {
		t.Fatalf("Records=%+v", listed.Records)
	}
	if listed.Records[0].SchemaVersion != StateCurrentSchemaVersion {
		t.Fatalf("SchemaVersion=%d want %d", listed.Records[0].SchemaVersion, StateCurrentSchemaVersion)
	}
}

func TestStateRejectsPathTraversalKeys(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, key := range []string{"", "../x", "x/y", "x\\y", "x..y"} {
		if _, err := StateWrite(key, "content"); err == nil {
			t.Fatalf("StateWrite(%q) succeeded; want error", key)
		}
	}
}

func TestStatePruneDryRunAndConfirm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	if _, err := StateWrite("old", "old content"); err != nil {
		t.Fatalf("StateWrite old: %v", err)
	}
	if _, err := StateWrite("fresh", "fresh content"); err != nil {
		t.Fatalf("StateWrite fresh: %v", err)
	}
	old, err := StateRead("old")
	if err != nil {
		t.Fatalf("StateRead old: %v", err)
	}
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	b, err := json.MarshalIndent(old.Record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old.Path, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	dry, err := StatePrune(time.Hour, false)
	if err != nil {
		t.Fatalf("StatePrune dry-run: %v", err)
	}
	if !dry.OK || !dry.DryRun || dry.Confirm || !containsString(dry.DeletedKeys, "old") || !containsString(dry.KeptKeys, "fresh") {
		t.Fatalf("unexpected dry-run prune result: %+v", dry)
	}
	if _, err := StateRead("old"); err != nil {
		t.Fatalf("dry-run removed old key: %v", err)
	}

	confirmed, err := StatePrune(time.Hour, true)
	if err != nil {
		t.Fatalf("StatePrune confirmed: %v", err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Confirm || !containsString(confirmed.DeletedKeys, "old") {
		t.Fatalf("unexpected confirmed prune result: %+v", confirmed)
	}
	if _, err := StateRead("old"); err == nil {
		t.Fatalf("old key still exists after confirmed prune")
	}
	if _, err := StateRead("fresh"); err != nil {
		t.Fatalf("fresh key removed unexpectedly: %v", err)
	}
}

func TestStatePruneRejectsInvalidMaxAge(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := StatePrune(0, false); err == nil {
		t.Fatalf("StatePrune accepted zero max age")
	}
}

func TestStateDoctorDetectsCorruptRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	if _, err := StateWrite("good", "good content"); err != nil {
		t.Fatalf("StateWrite good: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "badbytes.json"), []byte(`{"key":"badbytes","content":"abc","updated_at":"2000-01-01T00:00:00Z","bytes":999}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "badtime.json"), []byte(`{"key":"badtime","content":"abc","updated_at":"not-a-time","bytes":3}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

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
	if err := os.MkdirAll(filepath.Join(dir, "issueops-benchmarks"), 0o755); err != nil {
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
		if strings.Contains(issue.Path, "hook-failures.jsonl") || strings.Contains(issue.Path, "issueops-benchmarks") {
			t.Fatalf("harness-owned auxiliary state should not warn: %+v", result.Issues)
		}
	}
	for _, code := range []string{"unexpected_file", "unexpected_directory"} {
		if !stateDoctorHasIssue(result.Issues, code) {
			t.Fatalf("missing issue %s for unknown auxiliary state: %+v", code, result.Issues)
		}
	}
}

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
	if err := os.WriteFile(filepath.Join(dir, "legacy.json"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(filepath.Join(dir, "future.json"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
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

func docIndexContains(docs []DocIndexInfo, relPath string) bool {
	for _, doc := range docs {
		if doc.RelPath == relPath {
			return true
		}
	}
	return false
}

func stateDoctorHasIssue(issues []StateDoctorIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
