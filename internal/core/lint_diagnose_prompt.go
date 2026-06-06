package core

import "fmt"

func buildLintDiagnosePrompt(exitCode int, logTail string) string {
	return BuildStructuredPrompt(StructuredPromptSpec{
		Identity:  "You are a senior software development diagnoser.",
		Objective: fmt.Sprintf("Analyze the command execution output that resulted in a failure with exit code %d.", exitCode),
		Phases: []string{
			"Identify the root cause from the failure output.",
			"Determine the minimal concrete fix.",
			"Return a concise action-oriented diagnosis inside the response schema.",
		},
		Inputs: []string{"Command failure output tail."},
		Rules: []string{
			"Do not repeat the log.",
			"Do not speculate beyond the supplied output.",
			"Provide a code snippet only when it materially helps apply the fix.",
		},
		OutputContract: []string{
			"Return one JSON object matching the response schema.",
			"diagnosis must explain what went wrong and exactly how to fix it.",
			"diagnosis must be extremely concise, action-oriented, and focused.",
		},
		VerificationChecklist: []string{
			"The root cause is tied to evidence in the output.",
			"The fix is directly actionable.",
			"The response avoids log repetition.",
		},
		Data: []PromptDataSection{
			BuildExternalLLMJSONSchemaSection(lintDiagnoseResponseSchemaExample(), []string{
				"diagnosis: string, required, concise root cause and fix guidance.",
			}),
			{Title: "Execution Failure Output", Content: logTail},
		},
	})
}

func lintDiagnoseResponseSchemaExample() string {
	return `{
  "diagnosis": "Root cause tied to the log. Minimal fix to apply."
}`
}
