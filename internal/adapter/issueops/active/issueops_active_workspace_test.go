package active

import (
	"path/filepath"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestCycleForWorkspacePrefersWorktreeOverSourceRepo(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "child")

	store.writeRecord(t, model.IssueOpsRecord{
		ID: "io-parent", OK: true, Repo: repo, Branch: "1-parent",
		Phase: model.IssueOpsPhaseImplement,
	})
	store.writeRecord(t, model.IssueOpsRecord{
		ID: "io-child", OK: true, Repo: repo, Branch: "2-child",
		Phase: model.IssueOpsPhaseImplement, WorktreePath: worktree,
	})

	got, ok := CycleForWorkspace(store.issueOpsStore(), worktree)
	if !ok {
		t.Fatal("CycleForWorkspace(worktree) ok = false, want true")
	}
	if got.ID != "io-child" {
		t.Fatalf("CycleForWorkspace(worktree) = %q, want io-child", got.ID)
	}
}

func TestCycleForWorkspaceFallsBackToSourceRepo(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID: "io-source", OK: true, Repo: repo, Branch: "1-source",
		Phase: model.IssueOpsPhasePlan,
	})

	got, ok := CycleForWorkspace(store.issueOpsStore(), repo)
	if !ok || got.ID != "io-source" {
		t.Fatalf("CycleForWorkspace(repo) = %+v, %v; want io-source, true", got, ok)
	}
}

// done 사이클의 base_branch로 지금 열리는 MR을 판정하면 이미 정리된 브랜치를
// 요구하게 된다. 그래서 done은 조회 대상이 아니다.
func TestCycleForWorkspaceExcludesDoneCycles(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "merged")
	store.writeRecord(t, model.IssueOpsRecord{
		ID: "io-done", OK: true, Repo: repo, Branch: "1-done",
		Phase: model.IssueOpsPhaseDone, WorktreePath: worktree,
	})

	if _, ok := CycleForWorkspace(store.issueOpsStore(), worktree); ok {
		t.Fatal("CycleForWorkspace should skip done cycles")
	}
}

func TestCycleForWorkspaceRejectsEmptyPath(t *testing.T) {
	store := newActiveTestStore(t)
	if _, ok := CycleForWorkspace(store.issueOpsStore(), "   "); ok {
		t.Fatal("CycleForWorkspace(blank) ok = true, want false")
	}
}
