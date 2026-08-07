package prompt

import (
	"strings"
	"testing"
)

func TestBuildStructuredPromptUsesGoodPromptExampleShape(t *testing.T) {
	prompt := BuildStructuredPrompt(StructuredPromptSpec{
		Identity:              "You are a strict reviewer.",
		Objective:             "Review the supplied diff.",
		Phases:                []string{"Scan evidence.", "Apply rules."},
		Inputs:                []string{"Diff content."},
		Rules:                 []string{"Do not infer unrelated issues."},
		OutputContract:        []string{"Return JSON only."},
		VerificationChecklist: []string{"Every finding cites evidence."},
		Data:                  []PromptDataSection{{Title: "Diff", Content: "diff --git ..."}},
	})
	for _, heading := range StructuredPromptSectionHeadings {
		if !strings.Contains(prompt, heading) {
			t.Fatalf("structured prompt missing %q:\n%s", heading, prompt)
		}
	}
	if !strings.Contains(prompt, "## Diff") || !strings.Contains(prompt, "diff --git") {
		t.Fatalf("structured prompt should include data sections:\n%s", prompt)
	}
}
