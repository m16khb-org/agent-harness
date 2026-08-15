package issueopsretention

import (
	"context"
	"errors"
	"fmt"
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

	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("error = %v, want ErrIncomplete", err)
	}
	if result.OK {
		t.Fatal("retention with unreadable records must not report OK")
	}
	if len(result.Unreadable) != 1 ||
		result.Unreadable[0] != "broken" ||
		result.ReadErrors != 1 ||
		len(result.UnreadableDiagnostics) != 1 ||
		result.UnreadableDiagnostics[0].Code != "read_failed" ||
		len(result.Pruned) != 1 ||
		result.Pruned[0] != "old" {
		t.Fatalf("retention result = %+v", result)
	}
}

func TestServiceBoundsUnreadableDiagnosticsWithoutStoppingPrune(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 0, 0, 0, time.UTC)
	ids := make([]string, 0, unreadableReportLimit+6)
	readErrors := make(map[string]error, unreadableReportLimit+5)
	for index := 0; index < unreadableReportLimit+5; index++ {
		id := fmt.Sprintf("broken-%02d", index)
		ids = append(ids, id)
		readErrors[id] = errors.New("invalid state")
	}
	ids = append(ids, "old")
	repository := &fakeRepository{
		ids: ids,
		records: map[string]issueopscontract.IssueOpsRecord{
			"old": {
				ID:        "old",
				Phase:     issueopscontract.IssueOpsPhaseDone,
				UpdatedAt: now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
				Execution: &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased}},
			},
		},
		readErrors: readErrors,
	}

	result, err := NewService(repository, fixedClock{now: now}).Prune(context.Background(), "/state", 30*24*time.Hour, false)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("error = %v, want ErrIncomplete", err)
	}
	if result.ReadErrors != unreadableReportLimit+5 ||
		len(result.Unreadable) != unreadableReportLimit ||
		len(result.UnreadableDiagnostics) != unreadableReportLimit ||
		len(result.Pruned) != 1 || result.Pruned[0] != "old" {
		t.Fatalf("bounded retention result = %+v", result)
	}
}

func TestServiceReturnsPartialReceiptWhenLaterDeleteFails(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	old := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsCurrentSchemaVersion,
		ID:            "old",
		Phase:         issueopscontract.IssueOpsPhaseDone,
		UpdatedAt:     now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
	}
	later := old
	later.ID = "later"
	repository := &fakeRepository{
		ids:          []string{"old", "later"},
		records:      map[string]issueopscontract.IssueOpsRecord{"old": old, "later": later},
		deleteErrors: map[string]error{"later": errors.New("compare-and-swap failed")},
	}

	result, err := NewService(repository, fixedClock{now: now}).Prune(
		context.Background(),
		"/state",
		30*24*time.Hour,
		true,
	)

	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("error = %v", err)
	}
	if result.OK ||
		result.Error != ErrIncomplete.Error() ||
		result.DeleteErrors != 1 ||
		len(result.Pruned) != 1 || result.Pruned[0] != "old" ||
		len(result.Failed) != 1 || result.Failed[0] != "later" ||
		len(result.DeleteDiagnostics) != 1 || result.DeleteDiagnostics[0].Code != "delete_failed" {
		t.Fatalf("partial retention result = %+v", result)
	}
}

type fakeRepository struct {
	ids          []string
	records      map[string]issueopscontract.IssueOpsRecord
	readErrors   map[string]error
	deleteErrors map[string]error
	deleted      []string
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

func (repository *fakeRepository) DeleteIfUnchanged(_ context.Context, _ string, id string, _ issueopscontract.IssueOpsRecord) error {
	if err := repository.deleteErrors[id]; err != nil {
		return err
	}
	repository.deleted = append(repository.deleted, id)
	return nil
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}
