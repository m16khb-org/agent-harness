package core

import "agent-harness/internal/core/draftwiki"

const DraftWikiDir = draftwiki.DraftWikiDir

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
