package main

import (
	"agent-harness/internal/core"
	"flag"
	"fmt"
	"os"
)

func runProjectCommitSuggest(args []string) error {
	fs := flag.NewFlagSet("project commit-suggest", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	staged := fs.Bool("staged", false, "suggest commit based on staged changes (git diff --cached)")
	agyCommand := fs.String("agy-command", "agy", "Antigravity CLI executable")
	agyModel := fs.String("agy-model", "", "required agy settings.json model label; defaults to current settings model")
	agySettings := fs.String("agy-settings", "", "agy settings.json path; defaults to ~/.gemini/antigravity-cli/settings.json")
	jsonOut := fs.Bool("json", false, "print JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := core.SuggestCommit(core.CommitSuggestRequest{
		RepoRoot:        *repo,
		Staged:          *staged,
		AgyCommand:      *agyCommand,
		AgyModel:        *agyModel,
		AgySettingsPath: *agySettings,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(result)
	}

	if !result.Executed {
		fmt.Fprintln(os.Stderr, "No changes detected. Nothing to suggest.")
		return nil
	}

	fmt.Println(result.CommitMessage)
	return nil
}
