package issueopsstatus

import (
	"context"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestServiceReadsAndProjectsStatusWithoutPersisting(t *testing.T) {
	repository := &fakeRepository{
		record: issueopscontract.IssueOpsRecord{
			OK:    true,
			ID:    "io-status01",
			Phase: issueopscontract.IssueOpsPhasePlan,
		},
	}
	projector := &fakeProjector{}
	service := NewService(repository, projector)

	record, err := service.Status(context.Background(), "/state", "io-status01")
	if err != nil {
		t.Fatal(err)
	}
	if repository.reads != 1 || projector.projections != 1 {
		t.Fatalf("status must read and project once: reads=%d projections=%d", repository.reads, projector.projections)
	}
	if record.SourceMisdirectWarnings != 1 {
		t.Fatalf("projected record not returned: %+v", record)
	}
}

type fakeRepository struct {
	record issueopscontract.IssueOpsRecord
	reads  int
}

func (repository *fakeRepository) Read(context.Context, string, string) (issueopscontract.IssueOpsRecord, error) {
	repository.reads++
	return repository.record, nil
}

type fakeProjector struct {
	projections int
}

func (projector *fakeProjector) Project(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsRecord {
	projector.projections++
	record.SourceMisdirectWarnings++
	return record
}
