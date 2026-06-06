package main

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

func runIssueOps(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		issueOpsUsage()
		return nil
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("issueops start", flag.ContinueOnError)
		repo := fs.String("repo", "", "repository path")
		branch := fs.String("branch", "", "working branch")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: *repo, Branch: *branch})
		return printIssueOpsResult(record, *jsonOut, err)
	case "status":
		fs := flag.NewFlagSet("issueops status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-issue":
		fs := flag.NewFlagSet("issueops link-issue", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), *id, *issueURL)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-plan":
		fs := flag.NewFlagSet("issueops link-plan", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		planPath := fs.String("plan-path", "", "issue-driven plan path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), *id, *planPath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-worktree":
		fs := flag.NewFlagSet("issueops link-worktree", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		worktreePath := fs.String("worktree-path", "", "issue-driven worktree path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), *id, *worktreePath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-child":
		fs := flag.NewFlagSet("issueops link-child", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		childURL := fs.String("child-url", "", "GitHub sub-issue or GitLab child item URL")
		title := fs.String("title", "", "optional child issue title")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		if err := verifyIssueOpsChildIssueBeforeLink(*childURL); err != nil {
			return printIssueOpsResult(core.IssueOpsRecord{OK: false}, *jsonOut, err)
		}
		record, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), *id, *childURL, *title)
		return printIssueOpsResult(record, *jsonOut, err)
	case "branch":
		return runIssueOpsBranch(args[1:])
	case "worktree":
		return runIssueOpsWorktree(args[1:])
	case "phase":
		fs := flag.NewFlagSet("issueops phase", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		to := fs.String("to", "", "target phase: problem, grill, plan, implement, ai-slop-clean, feedback, pr, done")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), *id, *to)
		return printIssueOpsResult(record, *jsonOut, err)
	case "feedback":
		return runIssueOpsFeedback(args[1:])
	case "cleanup":
		return runIssueOpsCleanup(args[1:])
	case "benchmark":
		return runIssueOpsBenchmark(args[1:])
	case "remote":
		return runIssueOpsRemote(args[1:])
	case "remote-score":
		return runIssueOpsRemote(append([]string{"score"}, args[1:]...))
	case "pr-readiness":
		fs := flag.NewFlagSet("issueops pr-readiness", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		strict := fs.Bool("strict", false, "verify git cleanliness, upstream sync, plan path, and linked worktree path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		if err != nil {
			if *jsonOut {
				if printErr := printIssueOpsErrorJSON(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		readiness := core.IssueOpsPRReadiness(record)
		if *strict {
			readiness = core.IssueOpsStrictPRReadiness(record)
		}
		if *jsonOut {
			return printJSON(readiness)
		}
		fmt.Printf("ready: %v\n", readiness.Ready)
		for _, missing := range readiness.Missing {
			fmt.Printf("- missing: %s\n", missing)
		}
		return nil
	default:
		return fmt.Errorf("unknown issueops subcommand %q", args[0])
	}
}

func parseIssueOpsFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func issueOpsUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness issueops start --repo PATH [--branch NAME] [--json]
  agent-harness issueops status --id ID [--json]
  agent-harness issueops link-issue --id ID --issue-url URL [--json]
  agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--link-verified] [--json]
  agent-harness issueops link-plan --id ID --plan-path PATH [--json]
  agent-harness issueops link-worktree --id ID --worktree-path PATH [--json]
  agent-harness issueops worktree prepare-tools --id ID [--json]
  agent-harness issueops phase --id ID --to problem|grill|plan|implement|ai-slop-clean|feedback|pr|done [--json]
  agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] [--json]
  agent-harness issueops feedback mark-issue-updated --id ID [--json]
  agent-harness issueops pr-readiness --id ID [--strict] [--json]
  agent-harness issueops cleanup status --id ID [--merged] [--json]
  agent-harness issueops remote score --input PATH [--judge none|agy] [--json]
  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --label LABEL --assignee USER [--json]
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
