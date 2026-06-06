package core

import "agent-harness/internal/core/prompt"

type PromptDataSection = prompt.PromptDataSection
type StructuredPromptSpec = prompt.StructuredPromptSpec

var StructuredPromptSectionHeadings = prompt.StructuredPromptSectionHeadings

func BuildStructuredPrompt(spec StructuredPromptSpec) string {
	return prompt.BuildStructuredPrompt(spec)
}
