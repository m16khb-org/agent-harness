package draftwiki

func BuildDraftWikiSuggestPrompt(req DraftWikiSuggestRequest, input, targetType string) string {
	return buildDraftWikiSuggestPrompt(req, input, targetType)
}

func GeneratedDraftFrontmatter(title, targetWiki, targetType string) string {
	return generatedDraftFrontmatter(title, targetWiki, targetType)
}

func FailDraftWikiQueueEvent(event DraftWikiQueueEvent, err error) DraftWikiQueueEvent {
	return failDraftWikiQueueEvent(event, err)
}

func DraftWikiSeedFiles() map[string]string {
	return draftWikiSeedFiles()
}

type DraftWikiSuggestLLMResponse = draftWikiSuggestLLMResponse
