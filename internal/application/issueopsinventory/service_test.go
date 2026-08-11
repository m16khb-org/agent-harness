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
	if result.ScannedRecords != 4 {
		t.Fatalf("scanned records = %d, want 4", result.ScannedRecords)
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
		entry.LeaseStatus != string(issueopscontract.LeaseStatusClaimable) {
		t.Fatalf("claimable projection: %+v", entry)
	}
	if entry := entries["done"]; !entry.CleanupCandidate || !entry.CompletionUnreflected {
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

func (f fakeRepository) ListIDs(context.Context, string) ([]string, error) {
	return f.ids, f.listError
}

func (f fakeRepository) ReadUnchecked(_ context.Context, _ string, id string) (issueopsinventorycontract.Record, error) {
	if err := f.readErrors[id]; err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	return f.records[id], nil
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
