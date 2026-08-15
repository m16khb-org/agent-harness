package issueopsretention

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

func TestRepositoryReadsListsAndDeletesRecordWithStagedArtifact(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-retention01"
	record := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhaseDone,
		UpdatedAt:     "2026-06-01T00:00:00Z",
	}
	data, err := issueopsrecord.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: issueopsrecord.Bucket(), ID: id, Data: data},
		{Bucket: artifactStageBucket, ID: id, Data: []byte("staged")},
	}); err != nil {
		t.Fatal(err)
	}

	repository := Repository{}
	ids, err := repository.ListIDs(context.Background(), stateRoot)
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("list returned %+v, %v", ids, err)
	}
	got, err := repository.ReadUnchecked(context.Background(), stateRoot, id)
	if err != nil || !got.OK || got.ID != id {
		t.Fatalf("read returned %+v, %v", got, err)
	}
	if err := repository.DeleteIfUnchanged(context.Background(), stateRoot, id, got); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := database.Get(issueopsrecord.Bucket(), id); err != nil || ok {
		t.Fatalf("record survived delete: ok=%v err=%v", ok, err)
	}
	if _, ok, err := database.Get(artifactStageBucket, id); err != nil || ok {
		t.Fatalf("artifact survived delete: ok=%v err=%v", ok, err)
	}
	if _, err := repository.ReadUnchecked(context.Background(), stateRoot, id); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleted record read error = %v", err)
	}
}
