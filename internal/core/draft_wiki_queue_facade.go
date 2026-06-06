package core

import (
	"fmt"
	"path/filepath"

	"agent-harness/internal/core/draftwiki"
)

const draftWikiQueueFile = draftwiki.QueueFile
const draftWikiQueueLockFile = draftwiki.QueueLockFile
const maxDraftWikiQueueEvents = draftwiki.MaxQueueEvents

type DraftWikiQueueAppendRequest = draftwiki.DraftWikiQueueAppendRequest
type DraftWikiQueueEvent = draftwiki.DraftWikiQueueEvent
type DraftWikiQueueAppendResult = draftwiki.DraftWikiQueueAppendResult
type DraftWikiQueueProcessRequest = draftwiki.DraftWikiQueueProcessRequest
type DraftWikiQueueProcessResult = draftwiki.DraftWikiQueueProcessResult
type DraftWikiQueuePruneResult = draftwiki.DraftWikiQueuePruneResult
type DraftWikiQueuePruneAllResult = draftwiki.DraftWikiQueuePruneAllResult

func draftWikiQueuePath(repoRoot string, ensure bool) (ProjectLifecycleStatePlan, string, error) {
	plan, err := ValidateProjectLifecycleState(repoRoot)
	if err != nil {
		return plan, "", err
	}
	if ensure && !plan.Exists {
		plan, err = InitProjectLifecycleState(repoRoot, true)
		if err != nil {
			return plan, "", err
		}
	}
	if ensure && !plan.NamespaceValid {
		return plan, "", fmt.Errorf("project lifecycle namespace mismatch for %s", plan.RepoRoot)
	}
	return plan, filepath.Join(plan.ProjectStateDir, draftWikiQueueFile), nil
}

func trimDraftWikiQueueMaterial(material string) string {
	return draftwiki.TrimQueueMaterial(material)
}

func draftWikiQueueEventID(repoID, material, at string) string {
	return draftwiki.QueueEventID(repoID, material, at)
}

func acquireDraftWikiQueueLock(projectStateDir string) (func(), bool, error) {
	return draftwiki.AcquireQueueLock(projectStateDir)
}

func countDraftWikiQueueLines(path string, limit int) (int, error) {
	return draftwiki.CountQueueLines(path, limit)
}

func readDraftWikiQueueEvents(path string) ([]DraftWikiQueueEvent, []string, error) {
	return draftwiki.ReadQueueEvents(path)
}

func formatDraftWikiQueueMalformedWarning(lineNumber int, line string) string {
	return draftwiki.FormatQueueMalformedWarning(lineNumber, line)
}

func appendDraftWikiQueueEvent(path string, event DraftWikiQueueEvent) error {
	return draftwiki.AppendQueueEvent(path, event)
}

func capDraftWikiQueueEvents(path string, keep int) error {
	return draftwiki.CapQueueEvents(path, keep)
}

func pruneDraftWikiQueuePath(path string, keep int) (DraftWikiQueuePruneResult, error) {
	return draftwiki.PruneQueuePath(path, keep)
}

var rewriteDraftWikiQueueEventsFunc = func(path string, events []DraftWikiQueueEvent) error {
	return draftwiki.RewriteQueueEvents(path, events)
}

func rewriteDraftWikiQueueEvents(path string, events []DraftWikiQueueEvent) error {
	return draftwiki.RewriteQueueEvents(path, events)
}
