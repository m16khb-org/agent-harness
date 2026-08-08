package projectcli

import (
	commitsuggestcontract "agent-harness/internal/contract/commitsuggest"
	"flag"
	"fmt"
	"os"
)

func runProjectCommitSuggest(args []string) error {
	fs := flag.NewFlagSet("project commit-suggest", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	staged := fs.Bool("staged", false, "suggest commit based on staged changes (git diff --cached)")
	jsonOut := fs.Bool("json", false, "print JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := SuggestCommit(commitsuggestcontract.CommitSuggestRequest{
		RepoRoot: *repo,
		Staged:   *staged,
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

	fmt.Println(result.Prompt)
	return nil
}
