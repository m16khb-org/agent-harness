package issueopsinventory

import (
	"context"
	"errors"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

func TestServiceListCyclesFiltersAndProjectsInventory(t *testing.T) {
	now := time.Date(2026, time.August, 11, 3, 4, 5, 0, time.UTC)
	repository := fakeRepository{
		ids: []string{"planning", "claimable", "done", "elsewhere", "broken"},
		records: map[string]issueopscontract.IssueOpsRecord{
			"planning": {
				ID: "planning", Repo: "/repo", Branch: "84-planning",
				Phase: issueopscontract.IssueOpsPhasePlan,
			},
			"claimable": {
				ID: "claimable", Repo: "/repo", Branch: "84-claimable",
				Phase: issueopscontract.IssueOpsPhaseImplement,
				Execution: &issueopscontract.Execution{
					Mode: issueopscontract.ExecutionModeOrca,
					Workspace: issueopscontract.Workspace{
						Root: "/worktrees/84-claimable",
					},
					Lease: issueopscontract.WriteLease{
						Status: issueopscontract.LeaseStatusClaimable,
					},
					Pending: &issueopscontract.ExternalIntent{
						Kind:      "remote_pr_create",
						StartedAt: "2026-08-11T02:00:00Z",
					},
					Failure: &issueopscontract.ExecutionFailure{
						Code: "external_operation_ambiguous",
						At:   "2026-08-11T02:01:00Z",
					},
					Orca: &issueopscontract.OrcaBinding{
						OwnerModel: "gpt-5.6-terra",
					},
				},
			},
			"done": {
				ID: "done", Repo: "/repo", Branch: "84-done",
				Phase: issueopscontract.IssueOpsPhaseDone,
				RemoteArtifact: &issueopscontract.IssueOpsRemoteArtifactVerification{
					URL: "https://github.com/acme/repo/pull/1",
				},
				CleanupFinishFailure: &issueopscontract.IssueOpsCleanupFinishFailure{
					Step: "worktree_remove",
					At:   "2026-08-11T02:02:00Z",
				},
			},
			"elsewhere": {
				ID: "elsewhere", Repo: "/other", Phase: issueopscontract.IssueOpsPhasePlan,
			},
		},
		readErrors: map[string]error{"broken": errors.New("invalid record")},
	}
	service := NewService(repository, fixedClock{now: now}, cleanPath{})

	result, err := service.ListCycles(context.Background(), "/state", "/repo/")
	if err != nil {
		t.Fatalf("list cycles: %v", err)
	}
	if !result.OK || result.GeneratedAt != now.Format(time.RFC3339) {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	if result.ScannedRecords != 5 {
		t.Fatalf("scanned records = %d, want 5", result.ScannedRecords)
	}
	if result.ReadErrors != 1 ||
		len(result.UnreadableIDs) != 1 ||
		result.UnreadableIDs[0] != "broken" ||
		len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != "invalid_state" {
		t.Fatalf("unreadable records = %+v", result)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("entries = %d, want 3: %+v", len(result.Entries), result.Entries)
	}

	entries := make(map[string]issueopsinventorycontract.ListEntry, len(result.Entries))
	for _, entry := range result.Entries {
		entries[entry.ID] = entry
	}
	if entry := entries["planning"]; entry.Claimable || entry.CleanupCandidate {
		t.Fatalf("planning flags: %+v", entry)
	}
	if entry := entries["claimable"]; !entry.Claimable ||
		entry.OwnerModel != "gpt-5.6-terra" ||
		entry.LeaseStatus != string(issueopscontract.LeaseStatusClaimable) ||
		entry.PendingKind != "remote_pr_create" ||
		entry.FailureCode != "external_operation_ambiguous" {
		t.Fatalf("claimable projection: %+v", entry)
	}
	if entry := entries["done"]; !entry.CleanupCandidate ||
		!entry.CompletionUnreflected ||
		entry.CleanupFailureStep != "worktree_remove" {
		t.Fatalf("done projection: %+v", entry)
	}
}

func TestServiceListCyclesReturnsRepositoryFailure(t *testing.T) {
	want := errors.New("inventory unavailable")
	service := NewService(
		fakeRepository{listError: want},
		fixedClock{now: time.Unix(0, 0).UTC()},
		cleanPath{},
	)

	result, err := service.ListCycles(context.Background(), "/state", "")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if result.OK {
		t.Fatalf("failed result must not be ok: %+v", result)
	}
}

type fakeRepository struct {
	ids        []string
	records    map[string]issueopscontract.IssueOpsRecord
	readErrors map[string]error
	listError  error
}

func (f fakeRepository) Scan(
	context.Context,
	string,
) ([]issueopsinventorycontract.Record, []issueopsinventorycontract.RecordDiagnostic, error) {
	if f.listError != nil {
		return nil, nil, f.listError
	}
	records := make([]issueopsinventorycontract.Record, 0, len(f.ids))
	diagnostics := make([]issueopsinventorycontract.RecordDiagnostic, 0)
	for _, id := range f.ids {
		if f.readErrors[id] != nil {
			diagnostics = append(
				diagnostics,
				issueopsinventorycontract.RecordDiagnostic{ID: id, Code: "invalid_state"},
			)
			continue
		}
		records = append(records, f.records[id])
	}
	return records, diagnostics, nil
}

type fixedClock struct{ now time.Time }

func (f fixedClock) Now() time.Time { return f.now }

type cleanPath struct{}

func (cleanPath) Normalize(path string) string {
	for len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}
