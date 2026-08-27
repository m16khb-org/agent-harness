package active

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestNonDoneCyclesForRepoIncludesDeletedWorktreeAndExcludesDoneAndOtherRepos(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	other := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{ID: "io-a", OK: true, Repo: repo, Branch: "1-a", Phase: model.IssueOpsPhaseImplement, WorktreePath: filepath.Join(t.TempDir(), "gone")})
	store.writeRecord(t, model.IssueOpsRecord{ID: "io-b", OK: true, Repo: repo, Branch: "1-b", Phase: model.IssueOpsPhaseDone})
	store.writeRecord(t, model.IssueOpsRecord{ID: "io-c", OK: true, Repo: other, Branch: "1-c", Phase: model.IssueOpsPhasePlan})

	got := NonDoneCyclesForRepo(store.issueOpsStore(), repo)
	if len(got) != 1 || got[0].ID != "io-a" {
		t.Fatalf("NonDoneCyclesForRepo should return only the non-done cycle for repo (incl deleted worktree), got %+v", got)
	}
}

type activeTestStore struct {
	stateRoot string
	records   map[string]model.IssueOpsRecord
}

func newActiveTestStore(t *testing.T) *activeTestStore {
	t.Helper()
	return &activeTestStore{
		stateRoot: t.TempDir(),
		records:   map[string]model.IssueOpsRecord{},
	}
}

func (s *activeTestStore) issueOpsStore() Store {
	return Store{
		StateRoot: func() string {
			return s.stateRoot
		},
		Read: func(_ string, id string) (model.IssueOpsRecord, error) {
			return s.records[id], nil
		},
		NewID: func(string, string) string {
			return "io-active"
		},
		ListIDs: func(string) ([]string, error) {
			ids := make([]string, 0, len(s.records))
			for id := range s.records {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			return ids, nil
		},
	}
}

func (s *activeTestStore) writeRecord(t *testing.T, record model.IssueOpsRecord) {
	t.Helper()
	s.records[record.ID] = record
	b, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.stateRoot, record.ID+".json"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNonDoneCyclesUsesSingleBulkScan(t *testing.T) {
	scanCalls := 0
	store := Store{
		StateRoot: func() string { return "/state" },
		Scan: func(string) ([]model.IssueOpsRecord, error) {
			scanCalls++
			return []model.IssueOpsRecord{
				{ID: "io-match", Repo: "/repo", Phase: model.IssueOpsPhaseImplement},
				{ID: "io-done", Repo: "/repo", Phase: model.IssueOpsPhaseDone},
			}, nil
		},
		ListIDs: func(string) ([]string, error) {
			t.Fatal("bulk active scan must not list IDs")
			return nil, nil
		},
		Read: func(string, string) (model.IssueOpsRecord, error) {
			t.Fatal("bulk active scan must not read individual records")
			return model.IssueOpsRecord{}, nil
		},
	}

	records := NonDoneCyclesForRepo(store, "/repo")

	if scanCalls != 1 || len(records) != 1 || records[0].ID != "io-match" {
		t.Fatalf("scan calls=%d records=%+v", scanCalls, records)
	}
}
