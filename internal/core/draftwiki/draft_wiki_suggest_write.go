package draftwiki

import (
	"agent-harness/internal/core/draftwiki/suggestdraft"
)

func writeSuggestedDraft(root, title, targetWiki, targetType, agyModel, output string) (string, error) {
	return suggestdraft.Write(root, DraftWikiDir, title, targetWiki, targetType, agyModel, output)
}

func generatedDraftFrontmatter(title, targetWiki, targetType, agyModel string) string {
	return suggestdraft.Frontmatter(title, targetWiki, targetType, agyModel)
}

func stripMarkdownFence(output string) string {
	return suggestdraft.StripMarkdownFence(output)
}

func stripAgyOutputPreamble(output string) string {
	return suggestdraft.StripAgyOutputPreamble(output)
}

func slugifyDraftWiki(value string) string {
	return suggestdraft.Slugify(value)
}
