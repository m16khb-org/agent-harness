package core

import (
	"fmt"
	"path/filepath"

	"agent-harness/internal/core/draftwiki"
)

const DraftWikiDir = draftwiki.DraftWikiDir
const draftWikiQueueFile = draftwiki.QueueFile
const draftWikiQueueLockFile = draftwiki.QueueLockFile
const maxDraftWikiQueueEvents = draftwiki.MaxQueueEvents

type DraftWikiInitRequest = draftwiki.DraftWikiInitRequest
type DraftWikiInitResult = draftwiki.DraftWikiInitResult
type DraftWikiListRequest = draftwiki.DraftWikiListRequest
type DraftWikiListResult = draftwiki.DraftWikiListResult
type DraftWikiDraft = draftwiki.DraftWikiDraft
type DraftWikiMoveRequest = draftwiki.DraftWikiMoveRequest
type DraftWikiMoveResult = draftwiki.DraftWikiMoveResult
type DraftWikiPromoteRequest = draftwiki.DraftWikiPromoteRequest
type DraftWikiPromoteResult = draftwiki.DraftWikiPromoteResult
type DraftWikiSuggestRequest = draftwiki.DraftWikiSuggestRequest
type DraftWikiSuggestResult = draftwiki.DraftWikiSuggestResult
type draftWikiSuggestAgyResponse = draftwiki.DraftWikiSuggestAgyResponse
type DraftWikiQueueAppendRequest = draftwiki.DraftWikiQueueAppendRequest
type DraftWikiQueueEvent = draftwiki.DraftWikiQueueEvent
type DraftWikiQueueAppendResult = draftwiki.DraftWikiQueueAppendResult
type DraftWikiQueueProcessRequest = draftwiki.DraftWikiQueueProcessRequest
type DraftWikiQueueProcessResult = draftwiki.DraftWikiQueueProcessResult
type DraftWikiQueuePruneResult = draftwiki.DraftWikiQueuePruneResult
type DraftWikiQueuePruneAllResult = draftwiki.DraftWikiQueuePruneAllResult

func InitDraftWiki(req DraftWikiInitRequest) (DraftWikiInitResult, error) {
	return draftwiki.InitDraftWiki(req)
}

func ListDraftWiki(req DraftWikiListRequest) (DraftWikiListResult, error) {
	return draftwiki.ListDraftWiki(req)
}

func ApproveDraftWiki(req DraftWikiMoveRequest) (DraftWikiMoveResult, error) {
	return draftwiki.ApproveDraftWiki(req)
}

func RejectDraftWiki(req DraftWikiMoveRequest) (DraftWikiMoveResult, error) {
	return draftwiki.RejectDraftWiki(req)
}

func PromoteDraftWiki(req DraftWikiPromoteRequest) (DraftWikiPromoteResult, error) {
	return draftwiki.PromoteDraftWiki(req)
}

func SuggestDraftWiki(req DraftWikiSuggestRequest) (DraftWikiSuggestResult, error) {
	return draftwiki.SuggestDraftWiki(req)
}

func PruneDraftWikiQueue(repoRoot string, keep int) (DraftWikiQueuePruneResult, error) {
	return draftwiki.PruneDraftWikiQueue(repoRoot, keep)
}

func PruneAllDraftWikiQueues(stateDir string, keep int) (DraftWikiQueuePruneAllResult, error) {
	return draftwiki.PruneAllDraftWikiQueues(stateDir, keep)
}

func AppendDraftWikiQueueEvent(req DraftWikiQueueAppendRequest) (DraftWikiQueueAppendResult, error) {
	return draftwiki.AppendDraftWikiQueueEvent(req)
}

func ProcessDraftWikiQueue(req DraftWikiQueueProcessRequest) (DraftWikiQueueProcessResult, error) {
	previous := draftwiki.SetRewriteDraftWikiQueueEventsFunc(rewriteDraftWikiQueueEventsFunc)
	defer draftwiki.SetRewriteDraftWikiQueueEventsFunc(previous)
	return draftwiki.ProcessDraftWikiQueue(req)
}

func buildDraftWikiSuggestPrompt(req DraftWikiSuggestRequest, input, agyModel, targetType string) string {
	return draftwiki.BuildDraftWikiSuggestPrompt(req, input, agyModel, targetType)
}

func generatedDraftFrontmatter(title, targetWiki, targetType, agyModel string) string {
	return draftwiki.GeneratedDraftFrontmatter(title, targetWiki, targetType, agyModel)
}

func failDraftWikiQueueEvent(event DraftWikiQueueEvent, err error) DraftWikiQueueEvent {
	return draftwiki.FailDraftWikiQueueEvent(event, err)
}

func llmWikiRawNoteContent(draft DraftWikiDraft, targetType, today, draftContent string) string {
	return draftwiki.LLMWikiRawNoteContent(draft, targetType, today, draftContent)
}

func draftWikiRawFileName(today, draftPath string) string {
	return draftwiki.DraftWikiRawFileName(today, draftPath)
}

func draftWikiSeedFiles() map[string]string {
	return draftwiki.DraftWikiSeedFiles()
}

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
