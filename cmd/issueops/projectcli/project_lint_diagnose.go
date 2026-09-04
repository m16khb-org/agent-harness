package projectcli

import (
	"flag"
	"fmt"
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	"os"
)

func runProjectLintDiagnose(args []string) error {
	fs := flag.NewFlagSet("project lint-diagnose", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	commandArgv := fs.Args()
	if len(commandArgv) == 0 {
		return fmt.Errorf("missing command to run. Usage: issueops project lint-diagnose [flags] -- <command_to_run...>")
	}

	result, err := DiagnoseCommand(lintdiagnosecontract.LintDiagnoseRequest{
		RepoRoot:    *repo,
		CommandArgv: commandArgv,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(result)
	}

	if result.Failed {
		fmt.Fprintf(os.Stderr, "\n--- Host-Agent Diagnostic Prompt ---\n%s\n------------------------------------\n", result.Prompt)
		// Exit with the original exit code
		os.Exit(result.ExitCode)
	} else {
		fmt.Println("Command completed successfully. No failure detected.")
	}

	return nil
}
