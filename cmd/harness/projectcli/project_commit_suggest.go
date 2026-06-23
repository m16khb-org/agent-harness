package projectcli

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
	model := fs.String("model", "", "Z.AI model; defaults to glm-5-turbo")
	jsonOut := fs.Bool("json", false, "print JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := core.SuggestCommit(core.CommitSuggestRequest{
		RepoRoot: *repo,
		Staged:   *staged,
		Model:    *model,
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
