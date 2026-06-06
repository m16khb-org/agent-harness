package main

import (
	"flag"
	"fmt"

	"agent-harness/internal/core"
)

func runIssueOpsFeedback(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] [--json]\n       agent-harness issueops feedback mark-issue-updated --id ID [--json]")
		return nil
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("issueops feedback add", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		source := fs.String("source", "", "feedback source")
		body := fs.String("body", "", "feedback body")
		classification := fs.String("classification", "", "optional feedback classification, such as contract_change, defect, question, or noise")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.AddIssueOpsFeedback(core.IssueOpsStateRoot(), *id, *source, *body, *classification)
		return printIssueOpsResult(record, *jsonOut, err)
	case "mark-issue-updated":
		fs := flag.NewFlagSet("issueops feedback mark-issue-updated", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.MarkIssueOpsContractFeedbackIssueUpdated(core.IssueOpsStateRoot(), *id)
		return printIssueOpsResult(record, *jsonOut, err)
	default:
		return fmt.Errorf("unknown issueops feedback subcommand")
	}
}

func runIssueOpsCleanup(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops cleanup status --id ID [--merged] [--json]")
		return nil
	}
	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("issueops cleanup status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		merged := fs.Bool("merged", false, "confirm the remote PR/MR was verified merged before cleanup")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		verifiedMerged := issueOpsCleanupMerged(*id, *merged)
		status, err := core.IssueOpsCleanupStatusByID(core.IssueOpsStateRoot(), *id, core.IssueOpsCleanupStatusRequest{Merged: verifiedMerged})
		if err != nil {
			if *jsonOut {
				if printErr := printIssueOpsErrorJSON(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		if *jsonOut {
			return printJSON(status)
		}
		fmt.Printf("ready: %v\n", status.Ready)
		for _, missing := range status.Missing {
			fmt.Printf("- missing: %s\n", missing)
		}
		if len(status.Choices) > 0 {
			fmt.Println("선택지:")
			for _, choice := range status.Choices {
				fmt.Println(choice)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown issueops cleanup subcommand")
	}
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	if !requested {
		return false
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil || record.RemoteArtifact == nil {
		return false
	}
	return verifyIssueOpsRemoteArtifactMergedLive(*record.RemoteArtifact) == nil
}
