package issueopsinventory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
	statecontract "agent-harness/internal/contract/state"
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
	ids, err := repository.ListIDs(context.Background(), stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "io-invalid" || ids[1] != "io-valid" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	got, err := repository.ReadUnchecked(context.Background(), stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.ID != record.ID || got.Repo != record.Repo {
		t.Fatalf("unexpected record: %+v", got)
	}
	invalid, err := repository.ReadUnchecked(context.Background(), stateRoot, "io-invalid")
	if !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("invalid error = %v", err)
	}
	if !invalid.Invalid || invalid.InvalidReason == "" {
		t.Fatalf("invalid record must be explicit: %+v", invalid)
	}
}

func TestRepositoryHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repository := Repository{}
	if _, err := repository.ListIDs(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("list error = %v", err)
	}
	if _, err := repository.ReadUnchecked(ctx, t.TempDir(), "io-valid"); !errors.Is(err, context.Canceled) {
		t.Fatalf("read error = %v", err)
	}
}
