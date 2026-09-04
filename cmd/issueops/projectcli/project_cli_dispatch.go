package projectcli

import (
	"fmt"
	"os"
)

func runProject(args []string) error {
	if len(args) == 0 {
		projectUsage()
		return fmt.Errorf("missing project subcommand")
	}
	switch args[0] {
	case "bootstrap":
		return runProjectBootstrap(args[1:])
	case "docs":
		return runProjectDocs(args[1:])
	case "route-docs":
		return runProjectRouteDocs(args[1:])
	case "append":
		return runProjectAppend(args[1:])
	case "commit-suggest":
		return runProjectCommitSuggest(args[1:])
	case "lint-diagnose":
		return runProjectLintDiagnose(args[1:])
	default:
		projectUsage()
		return fmt.Errorf("unknown project subcommand %q", args[0])
	}
}

func projectUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops project bootstrap [--repo PATH] [--sync] [--dry-run] [--json]
  issueops project docs [--repo PATH] [--json]
  issueops project route-docs [--repo PATH] [--task TEXT] [--json]
  issueops project append --kind caution|adr --title TEXT --summary TEXT [--repo PATH] [--json]
  issueops project draft-wiki init|list|suggest|approve|reject|promote|prune ...
  issueops project commit-suggest [--repo PATH] [--staged] [--json]
  issueops project lint-diagnose [--repo PATH] [--json] -- <command_to_run...>
`)
}
