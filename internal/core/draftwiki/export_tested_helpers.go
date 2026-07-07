package draftwiki

import "agent-harness/internal/core/draftwiki/llmpromote"

func BuildDraftWikiSuggestPrompt(req DraftWikiSuggestRequest, input, targetType string) string {
	return buildDraftWikiSuggestPrompt(req, input, targetType)
}

func GeneratedDraftFrontmatter(title, targetWiki, targetType string) string {
	return generatedDraftFrontmatter(title, targetWiki, targetType)
}

func FailDraftWikiQueueEvent(event DraftWikiQueueEvent, err error) DraftWikiQueueEvent {
	return failDraftWikiQueueEvent(event, err)
}

func LLMWikiRawNoteContent(draft DraftWikiDraft, targetType, today, draftContent string) string {
	return llmpromote.RawNoteContent(llmPromoteDraft(draft), targetType, today, draftContent)
}

func DraftWikiRawFileName(today, draftPath string) string {
	return llmpromote.RawFileName(today, draftPath)
}

func DraftWikiSeedFiles() map[string]string {
	return draftWikiSeedFiles()
}

type DraftWikiSuggestLLMResponse = draftWikiSuggestLLMResponse
