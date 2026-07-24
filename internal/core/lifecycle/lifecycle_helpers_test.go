package lifecycle

import (
	"path/filepath"
	"testing"
)

func TestUniqueDocUpkeepEventsDeduplicatesByDocsAndSummary(t *testing.T) {
	events := []DocUpkeepEvent{
		{ID: "a", TargetDocs: []string{"OPERATIONS.md", "CONVENTIONS.md"}, Summary: "sync install docs", Source: "first"},
		{ID: "b", TargetDocs: []string{"CONVENTIONS.md", "OPERATIONS.md"}, Summary: " sync install docs ", Source: "duplicate"},
		{ID: "c", TargetDocs: []string{"TESTING.md"}, Summary: "sync install docs", Source: "distinct docs"},
		{ID: "d", TargetDocs: []string{"OPERATIONS.md"}, Summary: "record daemon behavior", Source: "distinct summary"},
	}

	got := uniqueDocUpkeepEvents(events)
	if len(got) != 3 {
		t.Fatalf("uniqueDocUpkeepEvents length = %d, want 3: %#v", len(got), got)
	}
	if got[0].ID != "a" || got[1].ID != "c" || got[2].ID != "d" {
		t.Fatalf("uniqueDocUpkeepEvents preserved IDs = %q, %q, %q; want a, c, d", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestWorktreeGuardEditTargetsResolvesExplicitPaths(t *testing.T) {
	repo := t.TempDir()
	absPath := filepath.Join(repo, "absolute.go")

	got := worktreeGuardEditTargets(HookToolUseLifecycleRequest{
		Repo:  repo,
		Paths: []string{"relative.go", absPath, "   "},
	})

	want := []string{
		filepath.Join(repo, "relative.go"),
		absPath,
	}
	if len(got) != len(want) {
		t.Fatalf("worktreeGuardEditTargets length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("worktreeGuardEditTargets[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// 경로가 전혀 해석되지 않는 mutation은 base(cwd/repo)를 타깃으로 대체해야
// 가드가 경로 없는 편집을 놓치지 않는다 — 폴백 분기의 안전성 회귀 방지.
func TestWorktreeGuardEditTargetsFallsBackToBase(t *testing.T) {
	repo := t.TempDir()
	cwd := t.TempDir()

	got := worktreeGuardEditTargets(HookToolUseLifecycleRequest{Repo: repo, CWD: cwd})
	if len(got) != 1 || got[0] != filepath.Clean(cwd) {
		t.Fatalf("empty paths must fall back to cwd: %#v", got)
	}

	got = worktreeGuardEditTargets(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "echo hello",
	})
	if len(got) != 1 || got[0] != filepath.Clean(repo) {
		t.Fatalf("pathless shell command must fall back to repo base: %#v", got)
	}
}
