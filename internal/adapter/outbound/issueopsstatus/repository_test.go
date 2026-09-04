package issueopsstatus

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestRepositoryReadsExistingRecordWithoutCreatingStore(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-status01"
	record := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhasePlan,
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueopsrecord.Bucket(), id, data); err != nil {
		t.Fatal(err)
	}

	got, err := (Repository{}).Read(context.Background(), stateRoot, id)
	if err != nil || !got.OK || got.ID != id {
		t.Fatalf("read returned %+v, %v", got, err)
	}
	if _, err := (Repository{}).Read(context.Background(), stateRoot, "io-missing01"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing record error = %v", err)
	}
}
