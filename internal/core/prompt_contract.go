package core

import (
	"strings"
)

type PromptDataSection struct {
	Title   string
	Content string
}

type StructuredPromptSpec struct {
	Identity              string
	Objective             string
	Phases                []string
	Inputs                []string
	Rules                 []string
	OutputContract        []string
	VerificationChecklist []string
	Data                  []PromptDataSection
}

var StructuredPromptSectionHeadings = []string{
	"## Identity",
	"## Objective",
	"## Operating Phases",
	"## Inputs",
	"## Rules",
	"## Output Contract",
	"## Verification Checklist",
}

func BuildStructuredPrompt(spec StructuredPromptSpec) string {
	var b strings.Builder
	writePromptSection(&b, "## Identity", spec.Identity)
	writePromptSection(&b, "## Objective", spec.Objective)
	writePromptListSection(&b, "## Operating Phases", spec.Phases)
	writePromptListSection(&b, "## Inputs", spec.Inputs)
	writePromptListSection(&b, "## Rules", spec.Rules)
	writePromptListSection(&b, "## Output Contract", spec.OutputContract)
	writePromptListSection(&b, "## Verification Checklist", spec.VerificationChecklist)
	for _, section := range spec.Data {
		title := strings.TrimSpace(section.Title)
		if title == "" {
			continue
		}
		writePromptSection(&b, "## "+title, section.Content)
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func writePromptSection(b *strings.Builder, heading, body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(heading)
	b.WriteString("\n\n")
	b.WriteString(body)
}

func writePromptListSection(b *strings.Builder, heading string, items []string) {
	var cleaned []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			cleaned = append(cleaned, item)
		}
	}
	if len(cleaned) == 0 {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(heading)
	b.WriteString("\n\n")
	for _, item := range cleaned {
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
}
