package projectcli

import (
	"fmt"
	"os"

	"agent-harness/cmd/harness/draftwikicli"
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
	case "record":
		return runProjectRecord(args[1:])
	case "draft-wiki":
		return draftwikicli.RunProjectDraftWiki(args[1:])
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
  agent-harness project bootstrap [--repo PATH] [--sync] [--dry-run] [--json]
  agent-harness project docs [--repo PATH] [--json]
  agent-harness project route-docs [--repo PATH] [--task TEXT] [--json]
  agent-harness project record --kind caution|adr --title TEXT --summary TEXT [--repo PATH] [--json]
  agent-harness project draft-wiki init|list|suggest|approve|reject|promote|prune ...
  agent-harness project commit-suggest [--repo PATH] [--staged] [--json]
  agent-harness project lint-diagnose [--repo PATH] [--json] -- <command_to_run...>
`)
}
