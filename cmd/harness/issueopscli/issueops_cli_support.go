package issueopscli

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

func issueOpsUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness issueops start --repo PATH [--branch NAME] [--json]
  agent-harness issueops status --id ID [--json]
  agent-harness issueops intent record --id ID --raw-request TEXT --interpreted-intent TEXT --success-criteria TEXT [--constraint TEXT] [--ambiguity TEXT] [--non-goal TEXT] [--json]
  agent-harness issueops link-issue --id ID --issue-url URL [--json]
  agent-harness issueops link-child --id ID --child-url URL [--title TEXT] [--json]
  agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--remote-branch-url URL] [--link-verified] [--json]
  agent-harness issueops link-worktree --id ID --worktree-path PATH [--json]
  agent-harness issueops design review --id ID --problem-summary TEXT --proposed-design TEXT --verification TEXT [--refactor-plan TEXT] [--alternative TEXT] [--risk TEXT] [--open-question TEXT] [--approved] [--json]
  agent-harness issueops link-plan --id ID --plan-path PATH [--json]
  agent-harness issueops worktree prepare-tools --id ID [--json]
  agent-harness issueops phase --id ID --to problem|grill|plan|implement|ai-slop-clean|feedback|pr|done [--json]
  agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] [--json]
  agent-harness issueops feedback mark-issue-updated --id ID [--json]
  agent-harness issueops pr-readiness --id ID [--strict] [--json]
  agent-harness issueops cleanup status --id ID [--merged] [--json]
  agent-harness issueops remote score --input PATH [--judge none|agy] [--agy-command PATH] [--json]
  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --label LABEL --assignee USER [--json]
  agent-harness issueops benchmark run --fixtures PATH [--judge none|agy] [--agy-command PATH] [--json]
  agent-harness issueops benchmark compare --baseline KEY --candidate KEY [--json]
  agent-harness issueops benchmark gate --baseline KEY --candidate KEY --candidate-file PATH [--changed-path PATH]... [--json]
`)
}

func runIssueOpsBranch(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--link-verified] [--json]")
		return nil
	}
	if args[0] != "prepare" {
		return fmt.Errorf("unknown issueops branch subcommand")
	}
	fs := flag.NewFlagSet("issueops branch prepare", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	provider := fs.String("provider", "", "remote provider: github or gitlab")
	issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
	branch := fs.String("branch", "", "provider-linked issue-number branch name")
	baseBranch := fs.String("base-branch", "", "remote base branch or ref")
	baseSHA := fs.String("base-sha", "", "optional resolved base commit SHA")
	remoteBranchURL := fs.String("remote-branch-url", "", "optional provider branch URL after creation")
	linkVerified := fs.Bool("link-verified", false, "record that the provider issue shows the branch link")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), *id, core.IssueOpsBranchPrepareRequest{
		Provider:        *provider,
		IssueURL:        *issueURL,
		Branch:          *branch,
		BaseBranch:      *baseBranch,
		BaseSHA:         *baseSHA,
		RemoteBranchURL: *remoteBranchURL,
		LinkVerified:    *linkVerified,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

func printIssueOpsResult(record core.IssueOpsRecord, jsonOut bool, err error) error {
	if err != nil {
		if jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if jsonOut {
		return printJSON(record)
	}
	fmt.Printf("%s %s %s\n", record.ID, record.Phase, record.Repo)
	return nil
}

func printIssueOpsErrorJSON(err error) error {
	if err == nil {
		return nil
	}
	return printJSON(map[string]any{
		"ok":    false,
		"error": err.Error(),
	})
}
