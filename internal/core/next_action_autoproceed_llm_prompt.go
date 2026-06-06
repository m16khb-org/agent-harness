package core

import (
	"fmt"
	"strings"
)

func buildNextActionAutoProceedLLMPrompt(recommended NextActionCandidate, candidates []NextActionCandidate) string {
	var choices strings.Builder
	for _, candidate := range candidates {
		marker := ""
		if candidate.Recommended {
			marker = " (recommended)"
		}
		fmt.Fprintf(&choices, "%d. %s%s\n", candidate.Index, candidate.Text, marker)
	}
	return BuildStructuredPrompt(StructuredPromptSpec{
		Identity:  "You are a cautious release-safety gate deciding whether an agent may auto-execute its recommended next action without asking the user.",
		Objective: "Given the explicitly recommended next action and the full list of choices, decide whether it is safe to AUTO-EXECUTE the recommended action without user confirmation.",
		Phases: []string{
			"Judge whether the recommended action is a confident, forward step.",
			"Judge whether it is reversible and free of external or irreversible side effects (no push, deploy, release, publish, merge, send, payment, infra apply, data drop).",
			"Set auto_proceed true only when the action is confident, forward, reversible, and side-effect-free; otherwise false.",
		},
		Inputs: []string{"The recommended next action.", "The full numbered choice list."},
		Rules: []string{
			"Auto-proceed only when there is no doubt the action is safe to run unattended.",
			"Any external, outbound, or irreversible side effect means auto_proceed must be false.",
			"Any ambiguity or uncertainty about intent means auto_proceed must be false.",
		},
		OutputContract: []string{
			"Return one JSON object matching the response schema.",
			"auto_proceed is true only when the recommended action is safe to auto-execute unattended.",
			"reason concisely justifies the decision.",
		},
		VerificationChecklist: []string{
			"auto_proceed is false whenever the action has any irreversible or external side effect.",
			"reason is grounded in the recommended action text.",
		},
		Data: []PromptDataSection{
			BuildExternalLLMJSONSchemaSection(nextActionAutoProceedLLMResponseSchemaExample(), []string{
				"auto_proceed: boolean, required, true only when safe to auto-execute unattended.",
				"reason: string, required, concise justification.",
			}),
			{Title: "Recommended Next Action", Content: recommended.Text},
			{Title: "All Next-Action Choices", Content: strings.TrimSpace(choices.String())},
		},
	})
}

func nextActionAutoProceedLLMResponseSchemaExample() string {
	return `{
  "auto_proceed": false,
  "reason": "Concise justification grounded in the recommended action."
}`
}
