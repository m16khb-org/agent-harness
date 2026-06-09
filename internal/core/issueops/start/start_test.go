package start

import (
	"testing"

	"agent-harness/internal/core/issueops/model"
)

type fakeStartStore struct {
	records map[string]model.IssueOpsRecord
	writes  []model.IssueOpsRecord
	valid   func(string) bool
}

func newFakeStartStore() *fakeStartStore {
	return &fakeStartStore{records: map[string]model.IssueOpsRecord{}}
}

func (s *fakeStartStore) store() Store {
	return Store{
		Read: func(_ string, id string) (model.IssueOpsRecord, error) {
			rec, ok := s.records[id]
			if !ok {
				return model.IssueOpsRecord{}, errNotFound
			}
			return rec, nil
		},
		Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			s.writes = append(s.writes, record)
			s.records[record.ID] = record
			return record, nil
		},
		NewID:          func(repo, branch string) string { return "io-fixed" },
		ValidateBranch: func(string) error { return nil },
		WorktreeValid:  s.valid,
	}
}

type startErr string

func (e startErr) Error() string { return string(e) }

const errNotFound = startErr("not found")

func TestStartResumesStaleCycleWhenValidatorNil(t *testing.T) {
	s := newFakeStartStore()
	s.valid = nil // fail-open: no liveness validator wired
	s.records["io-fixed"] = model.IssueOpsRecord{
		OK: true, ID: "io-fixed", Repo: "/repo", Branch: "1-x",
		Phase: model.IssueOpsPhaseImplement, WorktreePath: "/gone",
	}

	got, err := Start(s.store(), "/state", model.IssueOpsStartRequest{Repo: "/repo", Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != model.IssueOpsPhaseImplement || len(s.writes) != 0 {
		t.Fatalf("nil validator must resume without rewriting, got phase=%q writes=%d", got.Phase, len(s.writes))
	}
}

func TestStartResetWritesUnderSameRecordID(t *testing.T) {
	s := newFakeStartStore()
	s.valid = func(string) bool { return false } // worktree gone
	s.records["io-fixed"] = model.IssueOpsRecord{
		OK: true, ID: "io-fixed", Repo: "/repo", Branch: "1-x",
		Phase: model.IssueOpsPhaseImplement, WorktreePath: "/gone",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	got, err := Start(s.store(), "/state", model.IssueOpsStartRequest{Repo: "/repo", Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "io-fixed" || got.Phase != model.IssueOpsPhaseProblem {
		t.Fatalf("reset must keep id and become problem, got id=%q phase=%q", got.ID, got.Phase)
	}
	if got.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("reset must preserve CreatedAt for cycle-age identity, got %q", got.CreatedAt)
	}
	if len(s.writes) != 1 || s.writes[0].ID != "io-fixed" {
		t.Fatalf("reset must persist exactly one write under the same id, got %d writes", len(s.writes))
	}
}
