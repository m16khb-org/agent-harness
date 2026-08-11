package issueopsrecord

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestStoreReadsUpdatesRelatedDataAndDeletesAtomically(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-record01"
	data, err := json.Marshal(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(Bucket(), id, data); err != nil {
		t.Fatal(err)
	}

	store := Store{}
	record, err := store.Update(
		context.Background(),
		stateRoot,
		id,
		func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
			record.Branch = "record-store"
			return record, true, nil
		},
	)
	if err != nil || record.Branch != "record-store" {
		t.Fatalf("update returned %+v, %v", record, err)
	}
	if _, err := store.UpdateRelated(
		context.Background(),
		stateRoot,
		id,
		"related_v1",
		func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
			return []byte("related"), false, nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), stateRoot, id, "related_v1"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := database.Get(Bucket(), id); err != nil || found {
		t.Fatalf("record survived delete: found=%v err=%v", found, err)
	}
	if _, found, err := database.Get("related_v1", id); err != nil || found {
		t.Fatalf("related data survived delete: found=%v err=%v", found, err)
	}
}
