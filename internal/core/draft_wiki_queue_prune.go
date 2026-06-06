package core

import (
	"os"
	"path/filepath"
	"strings"
)

func PruneDraftWikiQueue(repoRoot string, keep int) (DraftWikiQueuePruneResult, error) {
	plan, path, err := draftWikiQueuePath(repoRoot, true)
	if err != nil {
		return DraftWikiQueuePruneResult{OK: false}, err
	}
	result, err := pruneDraftWikiQueuePath(path, keep)
	result.RepoRoot = plan.RepoRoot
	result.RepoID = plan.RepoID
	result.ProjectStateDir = plan.ProjectStateDir
	return result, err
}

func PruneAllDraftWikiQueues(stateDir string, keep int) (DraftWikiQueuePruneAllResult, error) {
	if strings.TrimSpace(stateDir) == "" {
		stateDir = StateDir()
	}
	result := DraftWikiQueuePruneAllResult{OK: true, Kind: "draft_wiki_queue_prune_all", StateDir: stateDir, Keep: keep, Queues: []DraftWikiQueuePruneResult{}}
	projectsDir := filepath.Join(stateDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		result.OK = false
		return result, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(projectsDir, entry.Name(), draftWikiQueueFile)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			result.OK = false
			return result, err
		}
		queue, err := pruneDraftWikiQueuePath(path, keep)
		queue.ProjectStateDir = filepath.Dir(path)
		if err != nil {
			result.OK = false
			return result, err
		}
		result.Before += queue.Before
		result.After += queue.After
		result.Pruned += queue.Pruned
		result.Queues = append(result.Queues, queue)
	}
	return result, nil
}
