package commitsuggest

import (
	"fmt"
	commitsuggestcontract "issueops/internal/contract/commitsuggest"
	"os/exec"
	"strings"

	"issueops/internal/domain/judgement"
	"issueops/internal/domain/prompt"
)

func SuggestCommit(req commitsuggestcontract.CommitSuggestRequest) (commitsuggestcontract.CommitSuggestResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return commitsuggestcontract.CommitSuggestResult{}, err
	}

	var gitArgs []string
	if req.Staged {
		gitArgs = []string{"-C", root, "diff", "--cached"}
	} else {
		gitArgs = []string{"-C", root, "diff"}
	}

	diffBytes, err := exec.Command("git", gitArgs...).Output()
	if err != nil {
		return commitsuggestcontract.CommitSuggestResult{}, fmt.Errorf("git diff failed: %w", err)
	}

	diffContent := string(diffBytes)
	if strings.TrimSpace(diffContent) == "" {
		return commitsuggestcontract.CommitSuggestResult{
			OK:       true,
			Executed: false,
			RepoRoot: root,
			Staged:   req.Staged,
		}, nil
	}

	return commitsuggestcontract.CommitSuggestResult{
		OK:       true,
		Executed: true,
		RepoRoot: root,
		Staged:   req.Staged,
		Prompt:   BuildPrompt(diffContent),
	}, nil
}

func BuildPrompt(diff string) string {
	return prompt.BuildStructuredPrompt(prompt.StructuredPromptSpec{
		Identity:  "You are an expert Git assistant.",
		Objective: "Read the git diff and draft one Conventional + Lore Hybrid commit message that strictly adheres to the project rules.",
		Phases: []string{
			"Identify the intent and scope of the diff.",
			"Choose the narrowest accurate Conventional Commit type and scope.",
			"Summarize the durable lore: intent, why, changes, verification, and risk.",
			"Return the commit message inside the response schema.",
		},
		Inputs: []string{"Git diff."},
		Rules: []string{
			"Subject format: <type>(<scope>)!?: <summary>.",
			"Subject summary must be imperative, plain English, and under 72 chars.",
			"Allowed types: feat, fix, docs, refactor, test, chore, ci, perf, style, revert.",
			"Lore body must strictly start with 'Lore:'.",
			"Lore body fields: Intent, Why, Changes, Verify, Risk.",
		},
		OutputContract: []string{
			"Return one JSON object matching the response schema.",
			"commit_message must contain only the raw commit message text.",
			"Do not output intro or outro text outside the JSON object or fenced json block.",
		},
		VerificationChecklist: []string{
			"Subject matches Conventional Commit syntax.",
			"Subject is under 72 characters.",
			"Lore body contains Intent, Why, Changes, Verify, and Risk.",
			"The message does not mention files or tests not supported by the diff.",
		},
		Data: []prompt.PromptDataSection{
			judgement.BuildJSONSchemaSection(commitSuggestResponseSchemaExample(), []string{
				"commit_message: string, required, the complete Conventional Commit subject and Lore body.",
			}),
			{Title: "Git Diff", Content: diff},
		},
	})
}

func commitSuggestResponseSchemaExample() string {
	return `{
  "commit_message": "fix(scope): concise imperative summary\n\nLore:\n- Intent: Explain the goal.\n- Why: Explain the context.\n- Changes:\n  - Summarize the main change.\n- Verify: Describe verified evidence.\n- Risk: Low."
}`
}
