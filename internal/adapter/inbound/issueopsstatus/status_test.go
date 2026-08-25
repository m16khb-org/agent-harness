package issueopsstatus

import (
	"context"
	"errors"
	"testing"

	issueopsstatusapplication "agent-harness/internal/application/issueopsstatus"
	issueopsstatuscontract "agent-harness/internal/contract/issueopsstatus"
)

type fakeReader struct {
	readCalls int
	stateRoot string
	id        string
	record    issueopsstatuscontract.Record
	err       error
}

func (reader *fakeReader) Read(_ context.Context, stateRoot, id string) (issueopsstatuscontract.Record, error) {
	reader.readCalls++
	reader.stateRoot = stateRoot
	reader.id = id
	return reader.record, reader.err
}

type markingProjector struct{ projectCalls int }

func (projector *markingProjector) Project(
	record issueopsstatuscontract.Record,
) issueopsstatuscontract.Record {
	projector.projectCalls++
	record.OK = true
	return record
}

func TestStatusHandlerProjectsStoredRecord(t *testing.T) {
	reader := &fakeReader{record: issueopsstatuscontract.Record{ID: "io-5"}}
	projector := &markingProjector{}
	handler := NewStatusHandler(issueopsstatusapplication.NewService(reader, projector))

	record, err := handler("/state", "io-5")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if reader.readCalls != 1 || reader.stateRoot != "/state" || reader.id != "io-5" {
		t.Fatalf("read received wrong call: calls=%d root=%q id=%q", reader.readCalls, reader.stateRoot, reader.id)
	}
	if projector.projectCalls != 1 {
		t.Fatalf("projector calls = %d, want 1", projector.projectCalls)
	}
	if !record.OK || record.ID != "io-5" {
		t.Fatalf("projected record mismatch: %+v", record)
	}
}

func TestStatusHandlerPropagatesReadError(t *testing.T) {
	reader := &fakeReader{err: errors.New("cycle not found")}
	projector := &markingProjector{}
	handler := NewStatusHandler(issueopsstatusapplication.NewService(reader, projector))

	if _, err := handler("/state", "io-missing"); err == nil {
		t.Fatal("read failure must surface through handler")
	}
	if projector.projectCalls != 0 {
		t.Fatal("projector must not run when the record cannot be read")
	}
}
