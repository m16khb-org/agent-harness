package active

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/adapter/issueops/pathutil"
	model "agent-harness/internal/contract/issueops"
	issueopsdomain "agent-harness/internal/domain/issueops"
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
	if record.Phase == model.IssueOpsPhaseDone {
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
	if !issueopsdomain.IssueOpsPhaseExpectsWorktree(record.Phase) {
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
		if record.Phase == model.IssueOpsPhaseDone {
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

// UmbrellaCycleForChildIssue returns the non-done cycle in the same repo that
// links the given issue URL as a provider-native child work item. It is how a
// child cycle discovers the umbrella branch its own work has to branch from and
// merge back into (#129).
//
// Done cycles are excluded on purpose: once the umbrella has merged and been
// cleaned up there is no branch left to target, and blocking a child on a
// vanished parent would strand it.
func UmbrellaCycleForChildIssue(store Store, repo, childIssueURL string) (model.IssueOpsRecord, bool) {
	childIssueURL = strings.TrimSpace(childIssueURL)
	if childIssueURL == "" {
		return model.IssueOpsRecord{}, false
	}
	for _, record := range NonDoneCyclesForRepo(store, repo) {
		for _, link := range record.IssueLinks {
			if link.Type != "child" || strings.TrimSpace(link.URL) != childIssueURL {
				continue
			}
			return record, true
		}
	}
	return model.IssueOpsRecord{}, false
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
