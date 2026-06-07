package issueops

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/pathutil"
)

func ActiveIssueOpsCycleForBranch(repo, branch string) (IssueOpsRecord, bool) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return IssueOpsRecord{}, false
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), newIssueOpsID(repo, branch))
	if err != nil {
		return IssueOpsRecord{}, false
	}
	if record.Phase == IssueOpsPhaseDone {
		return IssueOpsRecord{}, false
	}
	if issueOpsPlanBranchMismatchesRecord(record) {
		return IssueOpsRecord{}, false
	}
	return record, true
}

func ActiveIssueOpsLinkedWorktreeCycleForRepo(repo string) (IssueOpsRecord, bool) {
	records := ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
	if len(records) == 0 {
		return IssueOpsRecord{}, false
	}
	return records[0], true
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []IssueOpsRecord {
	repo = pathutil.CleanAbsPath(repo)
	if repo == "" {
		return nil
	}
	entries, err := os.ReadDir(IssueOpsStateRoot())
	if err != nil {
		return nil
	}
	records := []IssueOpsRecord{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		record, err := ReadIssueOps(IssueOpsStateRoot(), id)
		if err != nil {
			continue
		}
		if record.Phase == IssueOpsPhaseDone {
			continue
		}
		if issueOpsPlanBranchMismatchesRecord(record) {
			continue
		}
		worktree := strings.TrimSpace(record.WorktreePath)
		if worktree == "" || !issueOpsWorktreePathValid(worktree) {
			continue
		}
		recordRepo := pathutil.CleanAbsPath(record.Repo)
		recordWorktree := pathutil.CleanAbsPath(worktree)
		if recordRepo != repo && recordWorktree != repo && !pathutil.PathWithin(repo, recordWorktree) {
			continue
		}
		records = append(records, record)
	}
	return records
}

func issueOpsPlanBranchMismatchesRecord(record IssueOpsRecord) bool {
	planPath := pathutil.CleanAbsPath(record.PlanPath)
	repo := pathutil.CleanAbsPath(record.Repo)
	if planPath == "" || repo == "" || pathutil.PathWithin(planPath, repo) || !pathutil.IsInsideWorktreesPath(planPath) {
		return false
	}
	branch := gitBranchFromAncestor(planPath)
	return branch != "" && branch != strings.TrimSpace(record.Branch)
}

func gitBranchFromAncestor(path string) string {
	current := pathutil.CleanAbsPath(path)
	if current == "" {
		return ""
	}
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return pathutil.GitBranchFromHead(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
