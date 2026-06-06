package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateIssueOpsIsolatedWorktreePath(record IssueOpsRecord, path string) error {
	repo := cleanAbsPath(record.Repo)
	worktree := cleanAbsPath(path)
	if repo == "" || worktree == "" {
		return fmt.Errorf("worktree_path and repo must be absolute or resolvable paths")
	}
	if worktree == repo {
		return fmt.Errorf("worktree_path must be isolated from the source checkout")
	}
	parent := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees")
	if !pathWithin(worktree, parent) {
		return fmt.Errorf("worktree_path must be under sibling worktree directory: %s", parent)
	}
	info, err := os.Lstat(worktree)
	if err != nil {
		return fmt.Errorf("worktree_path does not exist: %s", worktree)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("worktree_path must not be a symlink: %s", worktree)
	}
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		return fmt.Errorf("source checkout path cannot be resolved: %s", repo)
	}
	resolvedWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return fmt.Errorf("worktree_path cannot be resolved: %s", worktree)
	}
	resolvedParent := filepath.Join(filepath.Dir(resolvedRepo), filepath.Base(resolvedRepo)+".worktrees")
	if resolvedWorktree == resolvedRepo {
		return fmt.Errorf("worktree_path must be isolated from the source checkout")
	}
	if !pathWithin(resolvedWorktree, resolvedParent) {
		return fmt.Errorf("worktree_path must resolve under sibling worktree directory: %s", resolvedParent)
	}
	return nil
}

func validateIssueOpsWorktreeBranch(record IssueOpsRecord, path string) error {
	expected := strings.TrimSpace(record.Branch)
	if expected == "" {
		return nil
	}
	actual := strings.TrimSpace(gitBranchFromHead(path))
	if actual == "" {
		return fmt.Errorf("worktree_path must be a git worktree on IssueOps branch %s", expected)
	}
	if actual != expected {
		return fmt.Errorf("worktree branch %s does not match IssueOps branch %s", actual, expected)
	}
	return nil
}
