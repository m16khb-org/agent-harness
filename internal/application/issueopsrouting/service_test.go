package issueopsrouting

import (
	"context"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestServiceRecordsIdempotentlyAndScoresObservedRouting(t *testing.T) {
	repository := &fakeRepository{
		record: issueopscontract.IssueOpsRecord{
			OK: true,
			ID: "io-routing01",
		},
	}
	service := NewService(
		repository,
		fixedClock{now: time.Date(2026, 8, 11, 5, 0, 0, 0, time.UTC)},
		matchingPaths{},
	)

	record, err := service.Record(
		context.Background(),
		"/state",
		"io-routing01",
		" plan ",
		" database-design ",
		issueopscontract.IssueOpsActor{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.RoutingTrace) != 1 || record.RoutingTrace[0].Phase != "plan" || record.RoutingTrace[0].Skill != "database-design" {
		t.Fatalf("routing entry was not normalized: %+v", record.RoutingTrace)
	}
	if repository.writes != 1 {
		t.Fatalf("first record must write once: %d", repository.writes)
	}

	if _, err := service.Record(
		context.Background(),
		"/state",
		"io-routing01",
		"PLAN",
		"DATABASE-DESIGN",
		issueopscontract.IssueOpsActor{},
	); err != nil {
		t.Fatal(err)
	}
	if repository.writes != 1 {
		t.Fatalf("duplicate routing entry must not rewrite: %d", repository.writes)
	}

	result, observed, err := service.Score(
		context.Background(),
		"/state",
		"io-routing01",
		[]issueopscontract.SkillRouting{
			{Phase: "plan", Skill: "database-design"},
			{Phase: "implement", Skill: "algorithm-optimization"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || observed != 1 || len(result.Missing) != 1 || result.Missing[0].Skill != "algorithm-optimization" {
		t.Fatalf("unexpected routing score: result=%+v observed=%d", result, observed)
	}
}

type fakeRepository struct {
	record issueopscontract.IssueOpsRecord
	writes int
}

func (repository *fakeRepository) Read(context.Context, string, string) (issueopscontract.IssueOpsRecord, error) {
	return repository.record, nil
}

func (repository *fakeRepository) Update(
	_ context.Context,
	_ string,
	_ string,
	mutate func(issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error),
) (issueopscontract.IssueOpsRecord, error) {
	next, changed, err := mutate(repository.record)
	if err != nil {
		return next, err
	}
	if changed {
		repository.record = next
		repository.writes++
	}
	return repository.record, nil
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
