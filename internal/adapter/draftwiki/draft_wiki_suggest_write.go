package draftwiki

import (
	"agent-harness/internal/adapter/draftwiki/suggestdraft"
)

func writeSuggestedDraft(root, title, targetWiki, targetType, output string) (string, error) {
	return suggestdraft.Write(root, DraftWikiDir, title, targetWiki, targetType, output)
}

func generatedDraftFrontmatter(title, targetWiki, targetType string) string {
	return suggestdraft.Frontmatter(title, targetWiki, targetType)
}
