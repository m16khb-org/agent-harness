package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CommitSuggestRequest struct {
	RepoRoot        string        `json:"repo_root"`
	Staged          bool          `json:"staged"`
	AgyCommand      string        `json:"agy_command"`
	AgyModel        string        `json:"agy_model"`
	AgySettingsPath string        `json:"-"`
	Timeout         time.Duration `json:"-"`
}

type CommitSuggestResult struct {
	OK            bool   `json:"ok"`
	Executed      bool   `json:"executed"`
	RepoRoot      string `json:"repo_root"`
	Staged        bool   `json:"staged"`
	CommitMessage string `json:"commit_message,omitempty"`
	AgyCommand    string `json:"agy_command"`
	AgyModel      string `json:"agy_model"`
}

func SuggestCommit(req CommitSuggestRequest) (CommitSuggestResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return CommitSuggestResult{}, err
	}

	// 1. Get git diff
	var gitArgs []string
	if req.Staged {
		gitArgs = []string{"-C", root, "diff", "--cached"}
	} else {
		gitArgs = []string{"-C", root, "diff"}
	}

	diffBytes, err := exec.Command("git", gitArgs...).Output()
	if err != nil {
		return CommitSuggestResult{}, fmt.Errorf("git diff failed: %w", err)
	}

	diffContent := string(diffBytes)
	if strings.TrimSpace(diffContent) == "" {
		return CommitSuggestResult{
			OK:       true,
			Executed: false,
			RepoRoot: root,
			Staged:   req.Staged,
		}, nil
	}

	// 2. Resolve agy settings & model
	agyCommand := strings.TrimSpace(req.AgyCommand)
	if agyCommand == "" {
		agyCommand = "agy"
	}
	settingsPath := resolveAgySettingsPath(req.AgySettingsPath)
	configuredModel, err := readAgyConfiguredModel(settingsPath)
	if err != nil {
		// Non-blocking fallback if settings file is missing or invalid
		configuredModel = "default"
	}
	agyModel := strings.TrimSpace(req.AgyModel)
	if agyModel == "" {
		agyModel = configuredModel
	}

	// 3. Compose prompt
	prompt := buildCommitSuggestPrompt(diffContent)

	// 4. Run agy
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, agyCommand, "-p", prompt)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return CommitSuggestResult{}, fmt.Errorf("agy commit suggest timed out after %s", timeout)
	}
	if err != nil {
		return CommitSuggestResult{}, fmt.Errorf("agy commit suggest failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return CommitSuggestResult{
		OK:            true,
		Executed:      true,
		RepoRoot:      root,
		Staged:        req.Staged,
		CommitMessage: strings.TrimSpace(string(out)),
		AgyCommand:    agyCommand,
		AgyModel:      agyModel,
	}, nil
}

func buildCommitSuggestPrompt(diff string) string {
	return BuildStructuredPrompt(StructuredPromptSpec{
		Identity:  "You are an expert Git assistant.",
		Objective: "Read the git diff and draft one Conventional + Lore Hybrid commit message that strictly adheres to the project rules.",
		Phases: []string{
			"Identify the intent and scope of the diff.",
			"Choose the narrowest accurate Conventional Commit type and scope.",
			"Summarize the durable lore: intent, why, changes, verification, and risk.",
			"Return only the raw commit message.",
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
			"Output only the raw commit message itself.",
			fmt.Sprintf("Do not include markdown code blocks such as %s.", "```"),
			"Do not output intro or outro text.",
		},
		VerificationChecklist: []string{
			"Subject matches Conventional Commit syntax.",
			"Subject is under 72 characters.",
			"Lore body contains Intent, Why, Changes, Verify, and Risk.",
			"The message does not mention files or tests not supported by the diff.",
		},
		Data: []PromptDataSection{{Title: "Git Diff", Content: diff}},
	})
}
