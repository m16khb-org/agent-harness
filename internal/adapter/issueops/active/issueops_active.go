package active

import (
	"strings"

	"agent-harness/internal/adapter/issueops/pathutil"
	model "agent-harness/internal/contract/issueops"
)

type Store struct {
	StateRoot func() string
	Read      func(stateRoot, id string) (model.IssueOpsRecord, error)
	Scan      func(stateRoot string) ([]model.IssueOpsRecord, error)
	NewID     func(repo, branch string) string
	ListIDs   func(stateRoot string) ([]string, error)
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
	inventory, err := scanActiveRecords(store, stateRoot)
	if err != nil {
		return nil
	}
	records := []model.IssueOpsRecord{}
	for _, record := range inventory {
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

func scanActiveRecords(store Store, stateRoot string) ([]model.IssueOpsRecord, error) {
	if store.Scan != nil {
		return store.Scan(stateRoot)
	}
	ids, err := store.ListIDs(stateRoot)
	if err != nil {
		return nil, err
	}
	records := make([]model.IssueOpsRecord, 0, len(ids))
	for _, id := range ids {
		record, readErr := store.Read(stateRoot, id)
		if readErr != nil {
			continue
		}
		records = append(records, record)
	}
	return records, nil
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
