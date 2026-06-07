package active

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
)

type Store struct {
	StateRoot func() string
	Read      func(stateRoot, id string) (model.IssueOpsRecord, error)
	NewID     func(repo, branch string) string
}

func CycleForBranch(store Store, repo, branch string) (model.IssueOpsRecord, bool) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return model.IssueOpsRecord{}, false
	}
	record, err := store.Read(store.StateRoot(), store.NewID(repo, branch))
	if err != nil {
		return model.IssueOpsRecord{}, false
	}
	if record.Phase == model.IssueOpsPhaseDone {
		return model.IssueOpsRecord{}, false
	}
	if planBranchMismatchesRecord(record) {
		return model.IssueOpsRecord{}, false
	}
	return record, true
}

func LinkedWorktreeCycleForRepo(store Store, repo string) (model.IssueOpsRecord, bool) {
	records := LinkedWorktreeCyclesForRepo(store, repo)
	if len(records) == 0 {
		return model.IssueOpsRecord{}, false
	}
	return records[0], true
}

func LinkedWorktreeCyclesForRepo(store Store, repo string) []model.IssueOpsRecord {
	repo = pathutil.CleanAbsPath(repo)
	if repo == "" {
		return nil
	}
	stateRoot := store.StateRoot()
	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		return nil
	}
	records := []model.IssueOpsRecord{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		record, err := store.Read(stateRoot, id)
		if err != nil {
			continue
		}
		if record.Phase == model.IssueOpsPhaseDone {
			continue
		}
		if planBranchMismatchesRecord(record) {
			continue
		}
		worktree := strings.TrimSpace(record.WorktreePath)
		if worktree == "" || !worktreePathValid(worktree) {
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

func planBranchMismatchesRecord(record model.IssueOpsRecord) bool {
	planPath := pathutil.CleanAbsPath(record.PlanPath)
	repo := pathutil.CleanAbsPath(record.Repo)
	if planPath == "" || repo == "" || pathutil.PathWithin(planPath, repo) || !pathutil.IsInsideWorktreesPath(planPath) {
		return false
	}
	branch := gitBranchFromAncestor(planPath)
	return branch != "" && branch != strings.TrimSpace(record.Branch)
}

func worktreePathValid(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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
