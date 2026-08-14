package issueopsinventory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestRepositoryListsAndStrictlyReadsRecords(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	record := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-valid",
		Repo:          "/repo",
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueopsrecord.Bucket(), record.ID, encoded); err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueopsrecord.Bucket(), "io-invalid", []byte(`{"schema_version":1,"id":"io-invalid","phase":"problem","unknown":true}`)); err != nil {
		t.Fatal(err)
	}

	repository := Repository{}
	records, diagnostics, err := repository.Scan(context.Background(), stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != record.ID || records[0].Repo != record.Repo {
		t.Fatalf("unexpected records: %+v", records)
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].ID != "io-invalid" ||
		diagnostics[0].Code != "invalid_state" {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
}

func TestRepositoryHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repository := Repository{}
	if _, _, err := repository.Scan(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("scan error = %v", err)
	}
}
