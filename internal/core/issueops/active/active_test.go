package active

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestLinkedWorktreeCycleForRepoReturnsFirstActiveRecord(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	worktree := t.TempDir()
	record := model.IssueOpsRecord{
		ID:           "io-active",
		OK:           true,
		Repo:         repo,
		Branch:       "1-active",
		IssueURL:     "https://github.com/example/repo/issues/1",
		WorktreePath: worktree,
		Phase:        model.IssueOpsPhaseImplement,
	}
	store.writeRecord(t, record)

	got, ok := LinkedWorktreeCycleForRepo(store.issueOpsStore(), repo)
	if !ok {
		t.Fatal("LinkedWorktreeCycleForRepo() ok = false, want true")
	}
	if got.ID != record.ID || got.WorktreePath != worktree {
		t.Fatalf("LinkedWorktreeCycleForRepo() = %+v, want id %s worktree %s", got, record.ID, worktree)
	}
}

func TestLinkedWorktreeCycleForRepoRejectsMissingRepo(t *testing.T) {
	store := newActiveTestStore(t)
	if got, ok := LinkedWorktreeCycleForRepo(store.issueOpsStore(), "   "); ok || got.ID != "" {
		t.Fatalf("LinkedWorktreeCycleForRepo(blank) = %+v, %v; want empty false", got, ok)
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
