package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func isLLMWikiRawType(rawType string) bool {
	switch rawType {
	case "articles", "papers", "repos", "notes", "data":
		return true
	default:
		return false
	}
}

func draftWikiRawFileName(today, draftPath string) string {
	base := strings.TrimSuffix(filepath.Base(draftPath), filepath.Ext(draftPath))
	base = slugifyDraftWiki(base)
	if base == "" {
		base = "draft-wiki-note"
	}
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}-`, base); matched {
		return base + ".md"
	}
	return today + "-" + base + ".md"
}

func llmWikiRawNoteContent(draft DraftWikiDraft, targetType, today, draftContent string) string {
	body := strings.TrimSpace(stripDraftWikiFrontmatter(draftContent))
	if body == "" {
		body = "# " + draft.Title + "\n"
	}
	summary := draft.Summary
	if summary == "" {
		summary = "Approved agent-harness draft wiki note promoted from repo-local review staging."
	}
	return fmt.Sprintf(`---
title: %q
source: %q
type: %s
ingested: %s
tags: [agent-harness, draft-wiki]
summary: %q
original_draft: %q
---

%s
`, draft.Title, "agent-harness draft-wiki:"+draft.RelPath, targetType, today, summary, draft.RelPath, body)
}

func stripDraftWikiFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}
