package main

import (
	"agent-harness/internal/core"
	"flag"
	"fmt"
	"os"
)

func runProjectLintDiagnose(args []string) error {
	fs := flag.NewFlagSet("project lint-diagnose", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	agyCommand := fs.String("agy-command", "agy", "Antigravity CLI executable")
	agyModel := fs.String("agy-model", "", "required agy settings.json model label; defaults to current settings model")
	agySettings := fs.String("agy-settings", "", "agy settings.json path; defaults to ~/.gemini/antigravity-cli/settings.json")
	jsonOut := fs.Bool("json", false, "print JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	commandArgv := fs.Args()
	if len(commandArgv) == 0 {
		return fmt.Errorf("missing command to run. Usage: agent-harness project lint-diagnose [flags] -- <command_to_run...>")
	}

	result, err := core.DiagnoseCommand(core.LintDiagnoseRequest{
		RepoRoot:        *repo,
		CommandArgv:     commandArgv,
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

	if result.Failed {
		fmt.Fprintf(os.Stderr, "\n--- Gemini 3.5 Flash Diagnostic Output ---\n%s\n------------------------------------------\n", result.Diagnosis)
		// Exit with the original exit code
		os.Exit(result.ExitCode)
	} else {
		fmt.Println("Command completed successfully. No failure detected.")
	}

	return nil
}
