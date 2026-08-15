package issueopsrecord

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestDeleteIfUnchangedRejectsRecordDrift(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-delete-cas"
	store := Store{}
	data, err := Encode(issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Repo:          t.TempDir(),
		Branch:        "100-before",
		Phase:         issueopscontract.IssueOpsPhaseDone,
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
	expected, err := store.Read(context.Background(), stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(context.Background(), stateRoot, id, func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		record.Branch = "100-after"
		return record, true, nil
	}); err != nil {
		t.Fatal(err)
	}

	err = store.DeleteIfUnchanged(context.Background(), stateRoot, id, expected, "artifact_stage_v1")

	if _, ok := errors.AsType[*sqlstore.RawCASError](err); !ok {
		t.Fatalf("error = %v, want RawCASError", err)
	}
	current, readErr := store.Read(context.Background(), stateRoot, id)
	if readErr != nil || current.Branch != "100-after" {
		t.Fatalf("current record = %+v, err = %v", current, readErr)
	}
}

func TestDeleteIfUnchangedSerializesNewRelatedState(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-delete-related"
	store := Store{}
	expected := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Repo:          t.TempDir(),
		Branch:        "101-related",
		Phase:         issueopscontract.IssueOpsPhaseDone,
	}
	data, err := Encode(expected)
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

	updateEntered := make(chan struct{})
	releaseUpdate := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		_, updateErr := store.UpdateRelated(
			context.Background(),
			stateRoot,
			id,
			"artifact_stage_v1",
			func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
				close(updateEntered)
				<-releaseUpdate
				return []byte(`{"plan":"new"}`), false, nil
			},
		)
		updateDone <- updateErr
	}()
	<-updateEntered

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- store.DeleteIfUnchanged(
			context.Background(),
			stateRoot,
			id,
			expected,
			"artifact_stage_v1",
		)
	}()
	close(releaseUpdate)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for name, result := range map[string]<-chan error{
		"related update":   updateDone,
		"retention delete": deleteDone,
	} {
		select {
		case operationErr := <-result:
			if operationErr != nil {
				t.Fatalf("%s: %v", name, operationErr)
			}
		case <-ctx.Done():
			t.Fatalf("%s did not finish: %v", name, ctx.Err())
		}
	}
	if _, found, getErr := database.Get(Bucket(), id); getErr != nil || found {
		t.Fatalf("record survived serialized delete: found=%v err=%v", found, getErr)
	}
	if _, found, getErr := database.Get("artifact_stage_v1", id); getErr != nil || found {
		t.Fatalf("related state survived serialized delete: found=%v err=%v", found, getErr)
	}
}

func TestStoreReadsUpdatesRelatedDataAndDeletesAtomically(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-record01"
	data, err := json.Marshal(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhaseProblem,
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

func TestStoreScansValidAndInvalidRowsInOneInventory(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	valid := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-valid",
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	}
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(Bucket(), valid.ID, encoded); err != nil {
		t.Fatal(err)
	}
	if err := database.Put(Bucket(), "io-invalid", []byte(`{"schema_version":1,"id":"io-invalid","phase":"unknown"}`)); err != nil {
		t.Fatal(err)
	}

	records, diagnostics, err := (Store{}).Scan(context.Background(), stateRoot)

	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != valid.ID {
		t.Fatalf("records = %+v", records)
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].ID != "io-invalid" ||
		diagnostics[0].Code != "invalid_state" {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}
