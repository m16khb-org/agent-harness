package active

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestLinkedWorktreeCycleForRepoReturnsFirstActiveRecord(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestSupervisedHandoffCyclesKeepsIdentifiableRecordOnEnvelopeReadError(t *testing.T) {
	repo := t.TempDir()
	worker := filepath.Join(repo+".worktrees", "future")
	record := model.IssueOpsRecord{
		ID: "io-future", Repo: repo, Branch: "future", Phase: model.IssueOpsPhaseImplement,
		ExecutionHandoff: &model.IssueOpsExecutionHandoff{ProtocolVersion: 99, State: "claimed", WorkerRoot: worker},
	}
	store := Store{
		StateRoot: func() string { return t.TempDir() },
		ListIDs:   func(string) ([]string, error) { return []string{record.ID, "io-unreadable"}, nil },
		Read: func(_ string, id string) (model.IssueOpsRecord, error) {
			if id == record.ID {
				return record, errors.New("invalid execution handoff envelope")
			}
			return model.IssueOpsRecord{}, errors.New("unreadable JSON")
		},
	}
	got := SupervisedHandoffCyclesForRepo(store, repo)
	if len(got) != 1 || got[0].ID != record.ID {
		t.Fatalf("identifiable version-skew handoff must retain guard authority: %#v", got)
	}
	if unrelated := SupervisedHandoffCyclesForRepo(store, t.TempDir()); len(unrelated) != 0 {
		t.Fatalf("invalid record for another repo must not cause a global block: %#v", unrelated)
	}
}

func TestSupervisedHandoffCyclesKeepsInvalidClosedV5PublicationAuthority(t *testing.T) {
	repo := t.TempDir()
	worker := filepath.Join(repo+".worktrees", "legacy")
	record := model.IssueOpsRecord{
		ID: "io-v5-publication", Repo: repo, Branch: "16-legacy", Phase: model.IssueOpsPhasePR,
		SchemaVersion: 5, Invalid: true,
		ExecutionHandoff: &model.IssueOpsExecutionHandoff{
			ProtocolVersion: 1, State: "closed", ClosedDisposition: "accepted", CoordinatorRoot: repo, WorkerRoot: worker,
			PublishReceipt: &model.IssueOpsExecutionHandoffPublishReceipt{},
		},
	}
	store := Store{
		StateRoot: func() string { return t.TempDir() },
		ListIDs:   func(string) ([]string, error) { return []string{record.ID}, nil },
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return record, errors.New("schema-v5 publication authority requires re-attestation")
		},
	}
	got := SupervisedHandoffCyclesForRepo(store, repo)
	if len(got) != 1 || got[0].ID != record.ID {
		t.Fatalf("invalid closed schema-v5 publication authority became invisible to hooks: %#v", got)
	}
}

func TestSupervisedHandoffCyclesKeepsInvalidV5CoordinatorAuthority(t *testing.T) {
	repo := t.TempDir()
	worker := filepath.Join(repo+".worktrees", "injected-v5")
	record := model.IssueOpsRecord{
		ID: "io-v5-coordinator", Repo: repo, Branch: "16-injected", Phase: model.IssueOpsPhaseImplement,
		SchemaVersion: 5, Invalid: true,
		ExecutionHandoff: &model.IssueOpsExecutionHandoff{
			ProtocolVersion: 1, State: "claimed", CoordinatorRoot: repo, WorkerRoot: worker,
			CoordinatorSession: &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "copied", AgentID: "copied"},
		},
	}
	store := Store{
		StateRoot: func() string { return t.TempDir() },
		ListIDs:   func(string) ([]string, error) { return []string{record.ID}, nil },
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return record, errors.New("schema_version 5 cannot contain coordinator_session durable mutation authority")
		},
	}
	got := SupervisedHandoffCyclesForRepo(store, repo)
	if len(got) != 1 || got[0].ID != record.ID {
		t.Fatalf("invalid v5 coordinator authority disappeared from lifecycle guard scan: %#v", got)
	}
}

func TestLinkedWorktreeCycleForRepoRejectsMissingRepo(t *testing.T) {
	store := newActiveTestStore(t)
	if got, ok := LinkedWorktreeCycleForRepo(store.issueOpsStore(), "   "); ok || got.ID != "" {
		t.Fatalf("LinkedWorktreeCycleForRepo(blank) = %+v, %v; want empty false", got, ok)
	}
}

func TestCycleForBranchRejectsWorktreePhaseWithMissingWorktreeDir(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID:           "io-active",
		OK:           true,
		Repo:         repo,
		Branch:       "1-active",
		Phase:        model.IssueOpsPhaseImplement,
		WorktreePath: filepath.Join(t.TempDir(), "deleted-worktree"),
	})

	if got, ok := CycleForBranch(store.issueOpsStore(), repo, "1-active"); ok || got.ID != "" {
		t.Fatalf("CycleForBranch() with deleted worktree on worktree-phase cycle = %+v, %v; want empty false", got, ok)
	}
}

func TestCycleForBranchKeepsWorktreePhaseWithLiveWorktreeDir(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	store.writeRecord(t, model.IssueOpsRecord{
		ID:           "io-active",
		OK:           true,
		Repo:         repo,
		Branch:       "1-active",
		Phase:        model.IssueOpsPhaseImplement,
		WorktreePath: worktree,
	})

	if got, ok := CycleForBranch(store.issueOpsStore(), repo, "1-active"); !ok || got.ID != "io-active" {
		t.Fatalf("CycleForBranch() with live worktree = %+v, %v; want io-active true", got, ok)
	}
}

func TestCycleForBranchKeepsNonWorktreePhaseWithoutWorktreeDir(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID:     "io-active",
		OK:     true,
		Repo:   repo,
		Branch: "1-active",
		Phase:  model.IssueOpsPhasePlan,
	})

	if got, ok := CycleForBranch(store.issueOpsStore(), repo, "1-active"); !ok || got.ID != "io-active" {
		t.Fatalf("CycleForBranch() on non-worktree phase without worktree = %+v, %v; want io-active true", got, ok)
	}
}

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

func TestWorktreeGitTracked(t *testing.T) {
	t.Run("git dir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if !worktreeGitTracked(dir) {
			t.Fatalf("directory with .git/ dir should be tracked, got false")
		}
	})
	t.Run("no git", func(t *testing.T) {
		dir := t.TempDir()
		if worktreeGitTracked(dir) {
			t.Fatalf("directory without .git should not be tracked, got true")
		}
	})
	t.Run("git file as linked worktree", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /some/real/git/dir\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !worktreeGitTracked(dir) {
			t.Fatalf("directory with .git file (linked worktree) should be tracked, got false")
		}
	})
}

func TestWorktreePhaseHasMissingWorktreeUsesGitTracked(t *testing.T) {
	dir := t.TempDir()
	rec := model.IssueOpsRecord{
		Phase:        model.IssueOpsPhaseImplement,
		WorktreePath: dir,
	}
	if !WorktreePhaseHasMissingWorktree(rec) {
		t.Fatal("non-git directory should be detected as missing worktree")
	}
}

func TestLinkedWorktreeCyclesExcludesNonGitDir(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	nonGitDir := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID:           "io-nongit",
		OK:           true,
		Repo:         repo,
		Branch:       "42-nongit",
		Phase:        model.IssueOpsPhaseImplement,
		WorktreePath: nonGitDir,
	})
	got := LinkedWorktreeCyclesForRepo(store.issueOpsStore(), repo)
	if len(got) != 0 {
		t.Fatalf("non-git worktree dir should be excluded from LinkedWorktreeCyclesForRepo, got %d records", len(got))
	}
}

func TestLinkedWorktreeCyclesIncludesGitFileWorktree(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	gitFileDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(gitFileDir, ".git"), []byte("gitdir: /some/real/git/dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.writeRecord(t, model.IssueOpsRecord{
		ID:           "io-gitfile",
		OK:           true,
		Repo:         repo,
		Branch:       "42-gitfile",
		Phase:        model.IssueOpsPhaseImplement,
		WorktreePath: gitFileDir,
	})
	got := LinkedWorktreeCyclesForRepo(store.issueOpsStore(), repo)
	if len(got) != 1 || got[0].ID != "io-gitfile" {
		t.Fatalf("linked worktree with .git file should be included, got %+v", got)
	}
}

func TestSupervisedHandoffCyclesRetainsOnlyNonterminalMissingWorktreeAuthority(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing-worker")
	store.writeRecord(t, model.IssueOpsRecord{
		ID: "io-supervised", Repo: repo, Branch: "42-supervised", Phase: model.IssueOpsPhaseImplement, WorktreePath: missing,
		ExecutionHandoff: &model.IssueOpsExecutionHandoff{State: "claimed", WorkerRoot: missing},
	})
	store.writeRecord(t, model.IssueOpsRecord{ID: "io-legacy", Repo: repo, Branch: "43-legacy", Phase: model.IssueOpsPhaseImplement, WorktreePath: filepath.Join(t.TempDir(), "legacy-missing")})
	store.writeRecord(t, model.IssueOpsRecord{
		ID: "io-closed", Repo: repo, Branch: "44-closed", Phase: model.IssueOpsPhaseImplement, WorktreePath: filepath.Join(t.TempDir(), "closed-missing"),
		ExecutionHandoff: &model.IssueOpsExecutionHandoff{State: "closed", WorkerRoot: filepath.Join(t.TempDir(), "closed-worker")},
	})
	got := SupervisedHandoffCyclesForRepo(store.issueOpsStore(), repo)
	if len(got) != 1 || got[0].ID != "io-supervised" {
		t.Fatalf("durable supervised lookup = %#v", got)
	}
}
