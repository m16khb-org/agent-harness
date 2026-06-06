package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

type draftWikiSuggestAgyResponse struct {
	BodyMarkdown string `json:"body_markdown"`
}

func buildDraftWikiSuggestPrompt(req DraftWikiSuggestRequest, input, agyModel, targetType string) string {
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
source: "claude-mem"
target_wiki: %q
target_type: %q
summary: "<one sentence>"
suggester: "agy -p"
model: %q
---`, title, targetWiki, targetType, agyModel),
		},
		VerificationChecklist: []string{
			"body_markdown has valid YAML frontmatter.",
			"The summary is one sentence.",
			"No secrets or transient logs are included.",
			"The document is reviewable as a repo-local draft.",
		},
		Data: []PromptDataSection{
			BuildExternalLLMJSONSchemaSection(draftWikiSuggestResponseSchemaExample(title, targetWiki, targetType, agyModel), []string{
				"body_markdown: string, required, the complete Markdown draft including YAML frontmatter.",
			}),
			{Title: "Source Material", Content: input},
		},
	})
}

func decodeDraftWikiSuggestAgyOutput(out []byte) (string, error) {
	var response draftWikiSuggestAgyResponse
	if err := DecodeExternalLLMStructuredJSONObject("agy draft wiki suggest", out, &response); err != nil {
		return "", fmt.Errorf("decode agy draft wiki output: %w", err)
	}
	body := strings.TrimSpace(response.BodyMarkdown)
	if body == "" {
		return "", fmt.Errorf("agy draft wiki output missing body_markdown")
	}
	return body, nil
}

func draftWikiSuggestResponseSchemaExample(title, targetWiki, targetType, agyModel string) string {
	body := fmt.Sprintf(`---
title: %q
source: "claude-mem"
target_wiki: %q
target_type: %q
summary: "One sentence summary."
suggester: "agy -p"
model: %q
---

# %s

Durable reusable knowledge goes here.`, title, targetWiki, targetType, agyModel, title)
	b, err := json.MarshalIndent(draftWikiSuggestAgyResponse{BodyMarkdown: body}, "", "  ")
	if err != nil {
		return `{"body_markdown":"---\ntitle: \"Draft wiki candidate\"\n---\n\n# Draft wiki candidate\n"}`
	}
	return string(b)
}
