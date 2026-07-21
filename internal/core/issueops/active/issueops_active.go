package active

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
)

type Store struct {
	StateRoot func() string
	Read      func(stateRoot, id string) (model.IssueOpsRecord, error)
	NewID     func(repo, branch string) string
	ListIDs   func(stateRoot string) ([]string, error)
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
	if record.Phase == model.IssueOpsPhaseDone && !retainsHandoffAuthority(record) {
		return model.IssueOpsRecord{}, false
	}
	if planBranchMismatchesRecord(record) {
		return model.IssueOpsRecord{}, false
	}
	if WorktreePhaseHasMissingWorktree(record) {
		return model.IssueOpsRecord{}, false
	}
	return record, true
}

// WorktreePhaseHasMissingWorktree reports whether a worktree-phase cycle points
// at a worktree directory that no longer exists. Such a cycle is stale: its
// isolated worktree was deleted without releasing the cycle, so it must not keep
// guard authority over the source checkout (which would otherwise deadlock all
// edits on its branch). An empty worktree path is a distinct, legitimate
// not-yet-linked state and is left untouched.
func WorktreePhaseHasMissingWorktree(record model.IssueOpsRecord) bool {
	if !model.IssueOpsPhaseExpectsWorktree(record.Phase) {
		return false
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		return false
	}
	return !worktreeGitTracked(worktree)
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
	ids, err := store.ListIDs(stateRoot)
	if err != nil {
		return nil
	}
	records := []model.IssueOpsRecord{}
	for _, id := range ids {
		record, err := store.Read(stateRoot, id)
		if err != nil {
			continue
		}
		if record.Phase == model.IssueOpsPhaseDone && !retainsHandoffAuthority(record) {
			continue
		}
		if planBranchMismatchesRecord(record) {
			continue
		}
		worktree := strings.TrimSpace(record.WorktreePath)
		if worktree == "" || !worktreeGitTracked(worktree) {
			continue
		}
		recordRepo := pathutil.CleanAbsPath(record.Repo)
		recordWorktree := pathutil.CleanAbsPath(worktree)
		if recordRepo != repo && recordWorktree != repo && !pathutil.PathWithin(repo, recordWorktree) {
			continue
		}
		records = append(records, record)
	}
	// Deterministic order so callers that surface or act on "the first" linked
	// cycle (e.g. the worktree-guard block message) are reproducible across
	// sessions rather than depending on os.ReadDir order. Sort by branch first
	// (the human-meaningful key), then by ID as a stable tiebreaker.
	sort.Slice(records, func(i, j int) bool {
		bi, bj := strings.TrimSpace(records[i].Branch), strings.TrimSpace(records[j].Branch)
		if bi != bj {
			return bi < bj
		}
		return records[i].ID < records[j].ID
	})
	return records
}

func retainsHandoffAuthority(record model.IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	return h != nil && h.State != "closed"
}

// NonDoneCyclesForRepo returns every non-done cycle whose record.Repo matches
// the given source repo, regardless of worktree validity. Unlike
// LinkedWorktreeCyclesForRepo it does NOT filter out cycles with a deleted or
// missing worktree — that is exactly the population the stale-cleanup scan needs
// to classify and prune.
func NonDoneCyclesForRepo(store Store, repo string) []model.IssueOpsRecord {
	repo = pathutil.CleanAbsPath(repo)
	if repo == "" {
		return nil
	}
	stateRoot := store.StateRoot()
	ids, err := store.ListIDs(stateRoot)
	if err != nil {
		return nil
	}
	records := []model.IssueOpsRecord{}
	for _, id := range ids {
		record, err := store.Read(stateRoot, id)
		if err != nil {
			continue
		}
		if record.Phase == model.IssueOpsPhaseDone {
			continue
		}
		if pathutil.CleanAbsPath(record.Repo) != repo {
			continue
		}
		records = append(records, record)
	}
	return records
}

// SupervisedHandoffCyclesForRepo keeps nonterminal durable handoff authority
// even when the linked worktree or its .git metadata has disappeared.
func SupervisedHandoffCyclesForRepo(store Store, repo string) []model.IssueOpsRecord {
	repo = pathutil.CleanAbsPath(repo)
	if repo == "" {
		return nil
	}
	stateRoot := store.StateRoot()
	ids, err := store.ListIDs(stateRoot)
	if err != nil {
		return nil
	}
	records := []model.IssueOpsRecord{}
	for _, id := range ids {
		record, err := store.Read(stateRoot, id)
		if record.ExecutionHandoff == nil {
			continue
		}
		if record.ExecutionHandoff.State == "closed" {
			continue
		}
		if err != nil && !safelyIdentifiableSupervisedHandoff(record) {
			continue
		}
		recordRepo := pathutil.CleanAbsPath(record.Repo)
		workerRoot := pathutil.CleanAbsPath(record.ExecutionHandoff.WorkerRoot)
		if recordRepo != repo && workerRoot != repo && !pathutil.PathWithin(repo, workerRoot) {
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Branch != records[j].Branch {
			return records[i].Branch < records[j].Branch
		}
		return records[i].ID < records[j].ID
	})
	return records
}

func safelyIdentifiableSupervisedHandoff(record model.IssueOpsRecord) bool {
	if record.ExecutionHandoff == nil {
		return false
	}
	repo := strings.TrimSpace(record.Repo)
	worker := strings.TrimSpace(record.ExecutionHandoff.WorkerRoot)
	if repo == "" || worker == "" || len(repo) > 4096 || len(worker) > 4096 || strings.ContainsRune(repo, 0) || strings.ContainsRune(worker, 0) {
		return false
	}
	return filepath.IsAbs(repo) && filepath.IsAbs(worker)
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

// worktreeGitTracked checks whether a path is a git-tracked directory — either a
// main checkout (where .git is a directory) or a linked worktree (where .git is
// a file pointing to the gitdir). Non-git directories and missing paths return
// false. Used on the hot path (worktree-guard and linked-worktree enumeration)
// to avoid false-live from `git worktree prune`d directories or leftover non-git
// dirs.
func worktreeGitTracked(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	_, err := os.Lstat(filepath.Join(path, ".git"))
	return err == nil
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
