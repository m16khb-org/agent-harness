package draftwikicli

import (
	"fmt"
	"os"
)

func runProjectDraftWiki(args []string) error {
	if len(args) == 0 {
		projectDraftWikiUsage()
		return fmt.Errorf("missing draft-wiki subcommand")
	}
	switch args[0] {
	case "init":
		return runProjectDraftWikiInit(args[1:])
	case "list":
		return runProjectDraftWikiList(args[1:])
	case "suggest":
		return runProjectDraftWikiSuggest(args[1:])
	case "submit":
		return runProjectDraftWikiSubmit(args[1:])
	case "queue":
		return runProjectDraftWikiQueue(args[1:])
	case "approve":
		return runProjectDraftWikiApprove(args[1:])
	case "reject":
		return runProjectDraftWikiReject(args[1:])
	case "promote":
		return runProjectDraftWikiPromote(args[1:])
	case "prune":
		return runProjectDraftWikiPrune(args[1:])
	default:
		projectDraftWikiUsage()
		return fmt.Errorf("unknown draft-wiki subcommand %q", args[0])
	}
}

func projectDraftWikiUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness project draft-wiki init [--repo PATH] [--dry-run] [--json]
  agent-harness project draft-wiki list [--repo PATH] [--json]
  agent-harness project draft-wiki suggest [--repo PATH] --input PATH [--title TITLE] [--target-wiki NAME] [--target-type notes] [--json]
  agent-harness project draft-wiki submit [--repo PATH] --draft PATH [--title TITLE] [--target-wiki NAME] [--target-type notes] [--json]
  agent-harness project draft-wiki queue [--repo PATH] (--input PATH|--material TEXT|--stdin) [--target-wiki NAME] [--target-type notes] [--json]
  agent-harness project draft-wiki approve [--repo PATH] [--json] PATH
  agent-harness project draft-wiki reject [--repo PATH] [--json] PATH
  agent-harness project draft-wiki promote [--repo PATH] [--target-wiki NAME] [--target-type notes] [--confirm] [--json] PATH
  agent-harness project draft-wiki prune [--repo PATH] [--all] [--keep N] [--json]
`)
}
