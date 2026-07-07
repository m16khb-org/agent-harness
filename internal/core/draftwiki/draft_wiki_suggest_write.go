package draftwiki

import (
	"agent-harness/internal/core/draftwiki/suggestdraft"
)

func writeSuggestedDraft(root, title, targetWiki, targetType, output string) (string, error) {
	return suggestdraft.Write(root, DraftWikiDir, title, targetWiki, targetType, output)
}

func generatedDraftFrontmatter(title, targetWiki, targetType string) string {
	return suggestdraft.Frontmatter(title, targetWiki, targetType)
}

func stripMarkdownFence(output string) string {
	return suggestdraft.StripMarkdownFence(output)
}

func stripLLMOutputPreamble(output string) string {
	return suggestdraft.StripLLMOutputPreamble(output)
}

func slugifyDraftWiki(value string) string {
	return suggestdraft.Slugify(value)
}
