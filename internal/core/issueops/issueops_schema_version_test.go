package issueops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core/sqlstore"
)

func TestLegacyIssueOpsRecordWithoutExecutionHandoffRemainsInline(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-legacy-inline"
	writeRawIssueOpsRecord(t, stateRoot, id, `{
  "ok": true,
  "id": "io-legacy-inline",
  "repo": "/repo/example",
  "branch": "16-demo",
  "phase": "implement",
  "created_at": "2026-07-11T00:00:00Z",
  "updated_at": "2026-07-11T00:00:00Z"
}`)

	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if record.ExecutionHandoff != nil {
		t.Fatalf("legacy record unexpectedly gained execution handoff: %#v", record.ExecutionHandoff)
	}
	readiness := IssueOpsImplementationReadiness(record)
	if containsString(readiness.Missing, "handoff_worker_claim") {
		t.Fatalf("legacy inline readiness must not require worker claim: %#v", readiness.Missing)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "execution_handoff") {
		t.Fatalf("legacy inline JSON contains execution_handoff: %s", encoded)
	}
}

func TestIssueOpsImplementationReadinessRequiresClaimOnlyForHandoff(t *testing.T) {
	record := IssueOpsRecord{ExecutionHandoff: &IssueOpsExecutionHandoff{
		State:          "dispatched",
		Attempt:        1,
		OwnershipEpoch: "epoch-1",
	}}
	readiness := IssueOpsImplementationReadiness(record)
	if !containsString(readiness.Missing, "handoff_worker_claim") {
		t.Fatalf("unclaimed handoff missing keys = %#v, want handoff_worker_claim", readiness.Missing)
	}
	record.ExecutionHandoff.State = "claimed"
	readiness = IssueOpsImplementationReadiness(record)
	if containsString(readiness.Missing, "handoff_worker_claim") {
		t.Fatalf("claimed handoff still requires worker claim: %#v", readiness.Missing)
	}
}

// writeRawIssueOpsRecord inserts record bytes directly into the state store,
// bypassing WriteIssueOps normalization, to simulate legacy or foreign rows.
func writeRawIssueOpsRecord(t *testing.T, stateRoot, id, raw string) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", id, []byte(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestIssueOpsReadNormalizesLegacySchemaVersion(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-legacy-schema"
	writeRawIssueOpsRecord(t, stateRoot, id, `{
  "ok": true,
  "id": "io-legacy-schema",
  "repo": "/repo/example",
  "branch": "1-demo",
  "phase": "problem",
  "created_at": "2026-07-02T00:00:00Z",
  "updated_at": "2026-07-02T00:00:00Z"
}
`)

	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != IssueOpsCurrentSchemaVersion {
		t.Fatalf("legacy record schema version = %d, want %d", record.SchemaVersion, IssueOpsCurrentSchemaVersion)
	}
}

func TestIssueOpsReadRejectsFutureSchemaVersion(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-future-schema"
	writeRawIssueOpsRecord(t, stateRoot, id, `{
  "ok": true,
  "schema_version": 4,
  "id": "io-future-schema",
  "repo": "/repo/example",
  "branch": "1-demo",
  "phase": "problem",
  "created_at": "2026-07-02T00:00:00Z",
  "updated_at": "2026-07-02T00:00:00Z"
}
`)

	_, err := ReadIssueOps(stateRoot, id)
	if err == nil || !strings.Contains(err.Error(), "unsupported issueops schema_version 4") {
		t.Fatalf("expected future schema rejection, got %v", err)
	}
}

func TestIssueOpsReadUpgradesV1HandoffWithoutDataLoss(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 1
	wantHandoff := *record.ExecutionHandoff
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}

	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != IssueOpsCurrentSchemaVersion || upgraded.ExecutionHandoff == nil || !reflect.DeepEqual(*upgraded.ExecutionHandoff, wantHandoff) {
		t.Fatalf("v1 handoff was not preserved during read migration: %#v", upgraded)
	}
	if _, err := WriteIssueOps(stateRoot, upgraded); err != nil {
		t.Fatal(err)
	}
	rewritten, ok, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !ok {
		t.Fatalf("read upgraded row: ok=%v err=%v", ok, err)
	}
	var stored map[string]any
	if err := json.Unmarshal(rewritten, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["schema_version"] != float64(IssueOpsCurrentSchemaVersion) || stored["execution_handoff"] == nil {
		t.Fatalf("v1 handoff write did not upgrade to current schema without data loss: %s", rewritten)
	}
}

func TestIssueOpsReadUpgradesV2HandoffWithoutDataLoss(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 2
	wantHandoff := *record.ExecutionHandoff
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}
	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != IssueOpsCurrentSchemaVersion || upgraded.ExecutionHandoff == nil || !reflect.DeepEqual(*upgraded.ExecutionHandoff, wantHandoff) {
		t.Fatalf("v2 handoff was not preserved during read migration: %#v", upgraded)
	}
}

func TestFutureSchemaReadPreservesBoundedHandoffIdentityAndInvalidMarker(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = IssueOpsCurrentSchemaVersion + 1
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}
	got, readErr := readIssueOpsUnchecked(stateRoot, record.ID)
	if readErr == nil || got.ExecutionHandoff == nil || got.ExecutionHandoff.WorkerRoot != record.ExecutionHandoff.WorkerRoot || !got.Invalid || got.InvalidReason == "" {
		t.Fatalf("future schema discarded bounded ownership identity: got=%#v err=%v", got, readErr)
	}
	if len(got.InvalidReason) > 256 || !strings.Contains(got.InvalidReason, "schema_version") {
		t.Fatalf("future schema invalid marker is unbounded or vague: %q", got.InvalidReason)
	}
}

func TestLegacyV1DecoderRejectsV2HandoffWithoutModifyingBytes(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 2
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}
	before, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyV1ReadModifyWriteFixture(stateRoot, record.ID); err == nil || !strings.Contains(err.Error(), "schema_version 2") {
		t.Fatalf("legacy v1 decoder did not reject v2 handoff: %v", err)
	}
	after, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("legacy v1 rejection modified durable bytes\nbefore=%s\n after=%s", before, after)
	}
}

func TestLegacyV2DecoderRejectsV3StableTerminalIdentityWithoutModifyingBytes(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	before, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyV2ReadModifyWriteFixture(stateRoot, record.ID); err == nil || !strings.Contains(err.Error(), "schema_version 3") {
		t.Fatalf("legacy v2 decoder did not reject v3 stable terminal identity: %v", err)
	}
	after, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("legacy v2 rejection modified durable bytes\nbefore=%s\n after=%s", before, after)
	}
}

func rawIssueOpsRecordBytes(stateRoot, id string) ([]byte, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	raw, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("missing issueops row %s", id)
	}
	return raw, nil
}

// legacyV1ReadModifyWriteFixture models the v1 decoder/writer boundary with a
// record type that deliberately does not know execution_handoff.
func legacyV1ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		OK            bool          `json:"ok"`
		SchemaVersion int           `json:"schema_version"`
		ID            string        `json:"id"`
		Repo          string        `json:"repo"`
		Branch        string        `json:"branch,omitempty"`
		Phase         IssueOpsPhase `json:"phase"`
		UpdatedAt     string        `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 1 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 1", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v1-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
}

func legacyV2ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 2 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 2", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v2-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
}

func TestIssueOpsWriteStampsCurrentSchemaVersion(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := WriteIssueOps(stateRoot, IssueOpsRecord{
		ID:    "io-current-schema",
		Repo:  "/repo/example",
		Phase: IssueOpsPhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != IssueOpsCurrentSchemaVersion {
		t.Fatalf("written record schema version = %d, want %d", record.SchemaVersion, IssueOpsCurrentSchemaVersion)
	}

	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.SchemaVersion != IssueOpsCurrentSchemaVersion {
		t.Fatalf("reloaded record schema version = %d, want %d", reloaded.SchemaVersion, IssueOpsCurrentSchemaVersion)
	}
}

func TestIssueOpsWriteRejectsFutureSchemaVersion(t *testing.T) {
	_, err := WriteIssueOps(t.TempDir(), IssueOpsRecord{
		ID:            "io-write-future-schema",
		SchemaVersion: IssueOpsCurrentSchemaVersion + 1,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported issueops schema_version") {
		t.Fatalf("expected future schema write rejection, got %v", err)
	}
}
