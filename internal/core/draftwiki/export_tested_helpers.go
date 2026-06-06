package draftwiki

func BuildDraftWikiSuggestPrompt(req DraftWikiSuggestRequest, input, agyModel, targetType string) string {
	return buildDraftWikiSuggestPrompt(req, input, agyModel, targetType)
}

func GeneratedDraftFrontmatter(title, targetWiki, targetType, agyModel string) string {
	return generatedDraftFrontmatter(title, targetWiki, targetType, agyModel)
}

func FailDraftWikiQueueEvent(event DraftWikiQueueEvent, err error) DraftWikiQueueEvent {
	return failDraftWikiQueueEvent(event, err)
}

func LLMWikiRawNoteContent(draft DraftWikiDraft, targetType, today, draftContent string) string {
	return llmWikiRawNoteContent(draft, targetType, today, draftContent)
}

func DraftWikiRawFileName(today, draftPath string) string {
	return draftWikiRawFileName(today, draftPath)
}

func DraftWikiSeedFiles() map[string]string {
	return draftWikiSeedFiles()
}

type DraftWikiSuggestAgyResponse = draftWikiSuggestAgyResponse
