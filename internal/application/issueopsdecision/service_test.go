package issueopsdecision

import (
	"context"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestServiceRecordsNormalizedDecision(t *testing.T) {
	repository := &fakeRepository{
		record: issueopscontract.IssueOpsRecord{OK: true, ID: "io-decision01"},
	}
	service := NewService(
		repository,
		fixedClock{now: time.Date(2026, 8, 11, 6, 0, 0, 0, time.UTC)},
		matchingPaths{},
	)

	record, err := service.Add(
		context.Background(),
		"/state",
		"io-decision01",
		issueopscontract.IssueOpsDecisionRecordRequest{
			Kind:  "architecture",
			Title: " Hexagonal boundary ",
			Body:  " Move decision recording into its own vertical. ",
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Decisions) != 1 {
		t.Fatalf("decision not recorded: %+v", record.Decisions)
	}
	decision := record.Decisions[0]
	if decision.Title != "Hexagonal boundary" ||
		decision.Body != "Move decision recording into its own vertical." ||
		decision.CreatedAt != "2026-08-11T06:00:00Z" {
		t.Fatalf("decision not normalized: %+v", decision)
	}
	if decision.Alternatives == nil ||
		decision.AffectedIssueLinks == nil ||
		decision.AffectedArtifacts == nil {
		t.Fatalf("optional slices must be stable empty arrays: %+v", decision)
	}
}

type fakeRepository struct {
	record issueopscontract.IssueOpsRecord
}

func (repository *fakeRepository) Update(
	_ context.Context,
	_ string,
	_ string,
	mutate func(issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error),
) (issueopscontract.IssueOpsRecord, error) {
	record, err := mutate(repository.record)
	if err != nil {
		return record, err
	}
	repository.record = record
	return record, nil
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

type matchingPaths struct{}

func (matchingPaths) Same(string, string) bool {
	return true
}
