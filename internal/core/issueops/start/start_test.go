package start

import (
	"encoding/json"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
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

func TestStartResetStampsOrphanWorktreePath(t *testing.T) {
	s := newFakeStartStore()
	s.valid = func(string) bool { return false } // worktree gone
	origWorktree := "/gone/worktree/path"
	s.records["io-fixed"] = model.IssueOpsRecord{
		OK: true, ID: "io-fixed", Repo: "/repo", Branch: "1-x",
		Phase: model.IssueOpsPhaseImplement, WorktreePath: origWorktree,
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	got, err := Start(s.store(), "/state", model.IssueOpsStartRequest{Repo: "/repo", Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.OrphanWorktreePath != origWorktree {
		t.Fatalf("stale reset must stamp orphan worktree path: want %q, got %q", origWorktree, got.OrphanWorktreePath)
	}
	if got.StaleResetAt == "" {
		t.Fatal("stale reset must set StaleResetAt")
	}
	if got.StaleResetPriorPhase != string(model.IssueOpsPhaseImplement) {
		t.Fatalf("StaleResetPriorPhase should be implement, got %q", got.StaleResetPriorPhase)
	}
}

func TestStartNeverStaleResetsExecutionHandoffLease(t *testing.T) {
	tests := []struct {
		name        string
		state       string
		disposition string
	}{
		{name: "ownership dispatching", state: handoff.StateOwnershipDispatching},
		{name: "recovery required", state: handoff.StateRecoveryRequired},
		{name: "ownership dispatched", state: handoff.StateOwnershipDispatched},
		{name: "owner orienting", state: handoff.StateOwnerOrienting},
		{name: "owner active", state: handoff.StateOwnerActive},
		{name: "cleanup pending", state: handoff.StateCleanupPendingHumanDecision},
		{name: "closed worker failed", state: handoff.StateClosed, disposition: handoff.DispositionWorkerFailed},
		{name: "closed cancelled", state: handoff.StateClosed, disposition: handoff.DispositionCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeStartStore()
			s.valid = func(string) bool { return false }
			record := model.IssueOpsRecord{
				OK: true, ID: "io-fixed", Repo: "/repo", Branch: "1-x",
				Phase: model.IssueOpsPhaseImplement, WorktreePath: "/gone/worktree",
				CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:01:00Z",
				ExecutionHandoff: &model.IssueOpsExecutionHandoff{
					State: tt.state, ClosedDisposition: tt.disposition,
					Attempt: 7, OwnershipEpoch: "epoch-preserved", AttemptBaseHead: "0123456789012345678901234567890123456789",
					Driver: "orca", Agent: "codex", CoordinatorRoot: "/repo", WorkerRoot: "/gone/worktree",
					Orca: &model.IssueOpsOrcaIdentity{WorktreeID: "wt-preserved", TaskID: "task-preserved", DispatchID: "dispatch-preserved"},
				},
			}
			s.records[record.ID] = record
			before, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}

			got, err := Start(s.store(), "/state", model.IssueOpsStartRequest{Repo: record.Repo, Branch: record.Branch})
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) || len(s.writes) != 0 {
				t.Fatalf("execution handoff lease changed during legacy stale reset: before=%s after=%s writes=%d", before, after, len(s.writes))
			}
		})
	}
}

func TestStartNeverStaleResetsExecutionWorkspaceAuthority(t *testing.T) {
	for _, state := range []string{"provisioning", "ready", handoff.StateRecoveryRequired} {
		t.Run(state, func(t *testing.T) {
			s := newFakeStartStore()
			s.valid = func(string) bool { return false }
			record := model.IssueOpsRecord{
				OK: true, ID: "io-fixed", Repo: "/repo", Branch: "1-x", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/gone/worktree", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:01:00Z",
				ExecutionWorkspace: &model.IssueOpsExecutionWorkspace{State: state, WorkspaceEpoch: "workspace-preserved", Driver: "orca", Agent: "codex", CoordinatorRoot: "/repo", WorkerRoot: "/gone/worktree", BaseHead: "0123456789012345678901234567890123456789"},
			}
			s.records[record.ID] = record
			before, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Start(s.store(), "/state", model.IssueOpsStartRequest{Repo: record.Repo, Branch: record.Branch})
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) || len(s.writes) != 0 {
				t.Fatalf("execution workspace authority changed during stale reset: before=%s after=%s writes=%d", before, after, len(s.writes))
			}
		})
	}
}
