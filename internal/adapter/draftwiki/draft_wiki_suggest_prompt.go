package draftwiki

import (
	draftwikicontract "agent-harness/internal/contract/draftwiki"
	"encoding/json"
	"fmt"
	"strings"
)

type draftWikiSuggestLLMResponse struct {
	BodyMarkdown string `json:"body_markdown"`
}

func buildDraftWikiSuggestPrompt(req draftwikicontract.DraftWikiSuggestRequest, input, targetType string) string {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Draft wiki candidate"
	}
	targetWiki := strings.TrimSpace(req.TargetWiki)
	if targetWiki == "" {
		targetWiki = "dev-fundamentals"
	}
	return BuildStructuredPrompt(StructuredPromptSpec{
		Identity:  "You are the agent-harness draft-wiki suggester.",
		Objective: "Turn the source material into one durable wiki draft if it contains reusable long-term knowledge.",
		Phases: []string{
			"Decide whether the source material contains reusable cross-session knowledge.",
			"Extract durable decisions, commands, paths, and cautions without copying transient noise.",
			"Write one reviewable Markdown draft with the required frontmatter.",
		},
		Inputs: []string{
			"Source material from the draft-wiki queue.",
			"Target wiki and target type metadata.",
		},
		Rules: []string{
			"Keep only reusable cross-session knowledge.",
			"Do not include secrets, credentials, transient logs, or private personal data.",
			"Preserve concrete commands, paths, and decisions when they are useful later.",
			`If the source is not worth remembering, put a short draft titled "Rejected: <reason>" in body_markdown explaining why.`,
		},
		OutputContract: []string{
			"Return one JSON object matching the response schema.",
			"body_markdown must contain exactly one Markdown document, no surrounding code fences.",
			fmt.Sprintf(`body_markdown should use this YAML frontmatter:
---
title: %q
source: "agent-notes"
target_wiki: %q
target_type: %q
summary: "<one sentence>"
suggester: "host-agent"
---`, title, targetWiki, targetType),
		},
		VerificationChecklist: []string{
			"body_markdown has valid YAML frontmatter.",
			"The summary is one sentence.",
			"No secrets or transient logs are included.",
			"The document is reviewable as a repo-local draft.",
		},
		Data: []PromptDataSection{
			BuildHostJudgementJSONSchemaSection(draftWikiSuggestResponseSchemaExample(title, targetWiki, targetType), []string{
				"body_markdown: string, required, the complete Markdown draft including YAML frontmatter.",
			}),
			{Title: "Source Material", Content: input},
		},
	})
}

func draftWikiSuggestResponseSchemaExample(title, targetWiki, targetType string) string {
	body := fmt.Sprintf(`---
title: %q
source: "agent-notes"
target_wiki: %q
target_type: %q
summary: "One sentence summary."
suggester: "host-agent"
---

# %s

Durable reusable knowledge goes here.`, title, targetWiki, targetType, title)
	b, err := json.MarshalIndent(draftWikiSuggestLLMResponse{BodyMarkdown: body}, "", "  ")
	if err != nil {
		return `{"body_markdown":"---\ntitle: \"Draft wiki candidate\"\n---\n\n# Draft wiki candidate\n"}`
	}
	return string(b)
}
