package readinesspaths

import (
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops/pathutil"
	model "issueops/internal/contract/issueops"
)

func StrictGitRoot(record model.IssueOpsRecord) string {
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		return path
	}
	return strings.TrimSpace(record.Repo)
}

func WorktreePathValid(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func PlanPathExists(repo, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(strings.TrimSpace(repo), path)
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func PlanInLinkedWorktree(record model.IssueOpsRecord) bool {
	planPath := strings.TrimSpace(record.PlanPath)
	worktree := strings.TrimSpace(record.WorktreePath)
	if planPath == "" || worktree == "" {
		return true
	}
	return PlanPathInsideWorktree(worktree, planPath)
}

func PlanPathInsideWorktree(worktree, planPath string) bool {
	planPath = strings.TrimSpace(planPath)
	if planPath == "" || strings.Contains(planPath, "\x00") {
		return false
	}
	if !filepath.IsAbs(planPath) {
		return true
	}
	if !pathutil.PathWithin(planPath, worktree) {
		return false
	}
	resolvedPlan, err := filepath.EvalSymlinks(planPath)
	if err != nil {
		return false
	}
	resolvedWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return false
	}
	return pathutil.PathWithin(resolvedPlan, resolvedWorktree)
}
