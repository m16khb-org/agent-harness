package active

import (
	"encoding/json"
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
