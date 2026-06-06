package issueops

import (
	"os"
	"path/filepath"
	"strings"
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
	repo = cleanAbsPath(repo)
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
		recordRepo := cleanAbsPath(record.Repo)
		recordWorktree := cleanAbsPath(worktree)
		if recordRepo != repo && recordWorktree != repo && !pathWithin(repo, recordWorktree) {
			continue
		}
		records = append(records, record)
	}
	return records
}

func issueOpsPlanBranchMismatchesRecord(record IssueOpsRecord) bool {
	planPath := cleanAbsPath(record.PlanPath)
	repo := cleanAbsPath(record.Repo)
	if planPath == "" || repo == "" || pathWithin(planPath, repo) || !isInsideWorktreesPath(planPath) {
		return false
	}
	branch := gitBranchFromAncestor(planPath)
	return branch != "" && branch != strings.TrimSpace(record.Branch)
}

func gitBranchFromAncestor(path string) string {
	current := cleanAbsPath(path)
	if current == "" {
		return ""
	}
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return gitBranchFromHead(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
