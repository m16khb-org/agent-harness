package issueopsrecord

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
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

	updateStarted := make(chan struct{})
	updateTxEntered := make(chan struct{})
	releaseUpdate := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		_, updateErr := store.UpdateRelated(
			context.Background(),
			stateRoot,
			id,
			"artifact_stage_v1",
			func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
				// 이 콜백은 이미 관련 트랜잭션(직렬화 게이트 + BEGIN IMMEDIATE)
				// 안에서 실행 중이다. delete가 먼저 커밋되고 이 Put이 되살리는
				// 순서 반전을 막으려면, delete는 이 트랜잭션이 커밋된 뒤에만
				// 게이트를 통과해야 한다. 게이트가 이미 releaseUpdate 대기로
				// 블록된 이 트랜잭션을 잡고 있으므로, delete 파생은 release 후에
				// 시작해야 순서가 보장된다.
				close(updateTxEntered)
				<-releaseUpdate
				return []byte(`{"plan":"new"}`), false, nil
			},
		)
		updateDone <- updateErr
	}()
	<-updateTxEntered
	close(updateStarted)

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

// DeleteIfUnchanged는 related update span이 열려 있는 동안 커밋해서는 안 된다.
// span이 직렬화 게이트를 쥔 채 콜백에서 블록돼 있으면 delete는 그 span이 끝날
// 때까지 기다려야 한다. 기다리지 않으면 delete가 먼저 커밋되고 span의 Put이
// related row를 되살려, 레코드는 사라졌는데 related state만 남는 고아가 된다.
func TestDeleteIfUnchangedWaitsForOpenRelatedUpdateSpan(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-delete-span"
	store := Store{}
	expected := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Repo:          t.TempDir(),
		Branch:        "102-span",
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

	spanEntered := make(chan struct{})
	releaseSpan := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		_, updateErr := store.UpdateRelated(
			context.Background(),
			stateRoot,
			id,
			"artifact_stage_v1",
			func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
				close(spanEntered)
				<-releaseSpan
				return []byte(`{"plan":"new"}`), false, nil
			},
		)
		updateDone <- updateErr
	}()
	<-spanEntered

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

	select {
	case deleteErr := <-deleteDone:
		t.Fatalf("delete committed while the related update span was still open: %v", deleteErr)
	case <-time.After(500 * time.Millisecond):
	}

	close(releaseSpan)
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
		t.Fatalf("record survived the delete: found=%v err=%v", found, getErr)
	}
	if _, found, getErr := database.Get("artifact_stage_v1", id); getErr != nil || found {
		t.Fatalf("related state survived the delete: found=%v err=%v", found, getErr)
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
