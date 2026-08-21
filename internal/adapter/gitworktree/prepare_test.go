package gitworktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/port"
)

// Prepare의 실제 계약을 잠근다: dry-run은 생성하지 않고, confirm은 형제
// .worktrees base에 실제 worktree를 만들며, 이미 있으면 identity가 일치할 때
// 통과하고 불일치하면 fail-closed로 거부한다. 정확히 dogfooding에서 밟았던
// 경로다(worktree 디렉터리 이름은 브랜치명과 일치해야 prepare가 만들 수 있다).
func TestPrepareCreatesCanonicalWorktreeOnConfirm(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-69", SourceRoot: repo,
		Root:   filepath.Join(base, "69-prepare"),
		Branch: "69-prepare", BaseBranch: "main",
		BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"),
		Confirm:  true,
	}
	if _, err := New().Prepare(context.Background(), req); err != nil {
		t.Fatalf("prepare confirm: %v", err)
	}
	if info, err := os.Lstat(req.Root); err != nil || !info.IsDir() {
		t.Fatalf("prepared worktree missing: %v", err)
	}
	if got := preflight.GitOut(req.Root, "branch", "--show-current"); got != req.Branch {
		t.Fatalf("prepared branch = %q want %q", got, req.Branch)
	}
	// 동일 요청 재실행은 기존 worktree identity로 통과해야 한다(멱등).
	receipt, err := New().Prepare(context.Background(), req)
	if err != nil || !receipt.Exists {
		t.Fatalf("re-prepare must be idempotent: receipt=%#v err=%v", receipt, err)
	}
}

func TestPrepareDryRunDoesNotCreateWorktree(t *testing.T) {
	repo := initAccessRepo(t)
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-70", SourceRoot: repo,
		Root:   repo + ".worktrees/70-dry",
		Branch: "70-dry", BaseBranch: "main",
		BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"),
	}
	receipt, err := New().Prepare(context.Background(), req)
	if err != nil {
		t.Fatalf("dry-run prepare: %v", err)
	}
	if receipt.Exists {
		t.Fatalf("dry-run must not report an existing worktree: %#v", receipt)
	}
	if _, err := os.Lstat(req.Root); !os.IsNotExist(err) {
		t.Fatalf("dry-run created a worktree: %v", err)
	}
}

func TestPrepareRejectsInvalidRequests(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	cases := []struct {
		name string
		req  port.ExecutionWorkspaceRequest
		want string
	}{
		{
			name: "missing identity fields",
			req:  port.ExecutionWorkspaceRequest{SourceRoot: repo, Root: filepath.Join(base, "x"), Branch: "x", BaseHead: "a"},
			want: "lifecycle_id, source_root, root, branch, and base_head are required",
		},
		{
			name: "non-sibling base",
			req: port.ExecutionWorkspaceRequest{
				LifecycleID: "io-71", SourceRoot: repo, Root: filepath.Join(filepath.Dir(repo), "elsewhere", "71-x"),
				Branch: "71-x", BaseBranch: "main", BaseHead: "a",
			},
			want: "sibling .worktrees base",
		},
		{
			// 검증 순서 계약: sibling-base 검사가 isolation 검사보다 먼저다.
			// root==source는 base 위반이기도 하므로 base 에러가 먼저 나온다.
			name: "root equals source",
			req: port.ExecutionWorkspaceRequest{
				LifecycleID: "io-72", SourceRoot: repo, Root: repo,
				Branch: "72-x", BaseBranch: "main", BaseHead: "a",
			},
			want: "sibling .worktrees base",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New().Prepare(context.Background(), tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPrepareRejectsOccupiedNonWorktreePath(t *testing.T) {
	repo := initAccessRepo(t)
	root := filepath.Join(repo+".worktrees", "73-occupied")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	// 일반 디렉터리(=git worktree가 아님)면 identity 검증이 실패해야 한다.
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-73", SourceRoot: repo, Root: root,
		Branch: "73-occupied", BaseBranch: "main",
		BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"),
		Confirm:  true,
	}
	if _, err := New().Prepare(context.Background(), req); err == nil {
		t.Fatal("occupied non-worktree path must fail closed")
	}
}
