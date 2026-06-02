package main

import (
	"flag"
	"fmt"

	"agent-harness/internal/core"
)

func runIssueOps(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand is required")
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("issueops start", flag.ContinueOnError)
		repo := fs.String("repo", "", "repository path")
		branch := fs.String("branch", "", "working branch")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: *repo, Branch: *branch})
		return printIssueOpsResult(record, *jsonOut, err)
	case "status":
		fs := flag.NewFlagSet("issueops status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-issue":
		fs := flag.NewFlagSet("issueops link-issue", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), *id, *issueURL)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-plan":
		fs := flag.NewFlagSet("issueops link-plan", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		planPath := fs.String("plan-path", "", "issue-driven plan path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), *id, *planPath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "feedback":
		return runIssueOpsFeedback(args[1:])
	case "pr-readiness":
		fs := flag.NewFlagSet("issueops pr-readiness", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		if err != nil {
			return err
		}
		readiness := core.IssueOpsPRReadiness(record)
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

func runIssueOpsFeedback(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return fmt.Errorf("unknown issueops feedback subcommand")
	}
	fs := flag.NewFlagSet("issueops feedback add", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	source := fs.String("source", "", "feedback source")
	body := fs.String("body", "", "feedback body")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	record, err := core.AddIssueOpsFeedback(core.IssueOpsStateRoot(), *id, *source, *body)
	return printIssueOpsResult(record, *jsonOut, err)
}

func printIssueOpsResult(record core.IssueOpsRecord, jsonOut bool, err error) error {
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(record)
	}
	fmt.Printf("%s %s %s\n", record.ID, record.Phase, record.Repo)
	return nil
}
