package issueopsretention

import (
	"context"
	"errors"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestServicePrunesOnlyProvablyRetainedDoneCycles(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 0, 0, 0, time.UTC)
	old := now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano)
	recent := now.Format(time.RFC3339Nano)
	released := issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased}
	active := issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusActive}
	repository := &fakeRepository{
		ids: []string{"old", "recent", "active", "unreflected", "reflected"},
		records: map[string]issueopscontract.IssueOpsRecord{
			"old": {
				ID:        "old",
				Phase:     issueopscontract.IssueOpsPhaseDone,
				UpdatedAt: old,
				Execution: &issueopscontract.Execution{Lease: released},
			},
			"recent": {
				ID:        "recent",
				Phase:     issueopscontract.IssueOpsPhaseDone,
				UpdatedAt: recent,
				Execution: &issueopscontract.Execution{Lease: released},
			},
			"active": {
				ID:        "active",
				Phase:     issueopscontract.IssueOpsPhaseDone,
				UpdatedAt: old,
				Execution: &issueopscontract.Execution{Lease: active},
			},
			"unreflected": {
				ID:             "unreflected",
				Phase:          issueopscontract.IssueOpsPhaseDone,
				UpdatedAt:      old,
				RemoteArtifact: &issueopscontract.IssueOpsRemoteArtifactVerification{},
			},
			"reflected": {
				ID:               "reflected",
				Phase:            issueopscontract.IssueOpsPhaseDone,
				UpdatedAt:        old,
				RemoteArtifact:   &issueopscontract.IssueOpsRemoteArtifactVerification{},
				RemoteCompletion: &issueopscontract.IssueOpsRemoteCompletion{ReflectedAt: "2026-08-01T00:00:00Z"},
			},
		},
	}
	service := NewService(repository, fixedClock{now: now})

	preview, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.Pruned) != 2 || preview.Pruned[0] != "old" || preview.Pruned[1] != "reflected" {
		t.Fatalf("preview selected wrong records: %+v", preview)
	}
	if len(repository.deleted) != 0 {
		t.Fatalf("preview deleted records: %v", repository.deleted)
	}

	result, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.DryRun || len(repository.deleted) != 2 || repository.deleted[0] != "old" || repository.deleted[1] != "reflected" {
		t.Fatalf("confirmed prune deleted wrong records: result=%+v deleted=%v", result, repository.deleted)
	}
}

func TestServiceRejectsNonPositiveMaxAge(t *testing.T) {
	service := NewService(&fakeRepository{}, fixedClock{now: time.Now()})
	if _, err := service.Prune(context.Background(), "/state", 0, false); err == nil {
		t.Fatal("max age must be positive")
	}
}

func TestServiceReportsUnreadableRecordsAndContinues(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		ids: []string{"broken", "old"},
		records: map[string]issueopscontract.IssueOpsRecord{
			"old": {
				ID:        "old",
				Phase:     issueopscontract.IssueOpsPhaseDone,
				UpdatedAt: now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
				Execution: &issueopscontract.Execution{
					Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased},
				},
			},
		},
		readErrors: map[string]error{"broken": errors.New("invalid state")},
	}
	service := NewService(repository, fixedClock{now: now})

	result, err := service.Prune(context.Background(), "/state", 30*24*time.Hour, false)

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Unreadable) != 1 ||
		result.Unreadable[0] != "broken" ||
		len(result.Pruned) != 1 ||
		result.Pruned[0] != "old" {
		t.Fatalf("retention result = %+v", result)
	}
}

type fakeRepository struct {
	ids        []string
	records    map[string]issueopscontract.IssueOpsRecord
	readErrors map[string]error
	deleted    []string
}

func (repository *fakeRepository) ListIDs(context.Context, string) ([]string, error) {
	return repository.ids, nil
}

func (repository *fakeRepository) ReadUnchecked(_ context.Context, _ string, id string) (issueopscontract.IssueOpsRecord, error) {
	if err := repository.readErrors[id]; err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	return repository.records[id], nil
}

func (repository *fakeRepository) Delete(_ context.Context, _ string, id string) error {
	repository.deleted = append(repository.deleted, id)
	return nil
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}
