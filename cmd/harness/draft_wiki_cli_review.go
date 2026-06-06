package main

import (
	"flag"
	"fmt"

	"agent-harness/internal/core"
)

func runProjectDraftWikiApprove(args []string) error {
	path, repo, jsonOut, err := parseDraftWikiPathFlags("project draft-wiki approve", args)
	if err != nil {
		return err
	}
	result, err := core.ApproveDraftWiki(core.DraftWikiMoveRequest{RepoRoot: repo, Path: path})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(result)
	}
	fmt.Printf("approved draft: %s -> %s\n", result.From.RelPath, result.To.RelPath)
	return nil
}

func runProjectDraftWikiReject(args []string) error {
	path, repo, jsonOut, err := parseDraftWikiPathFlags("project draft-wiki reject", args)
	if err != nil {
		return err
	}
	result, err := core.RejectDraftWiki(core.DraftWikiMoveRequest{RepoRoot: repo, Path: path})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(result)
	}
	fmt.Printf("rejected draft: %s -> %s\n", result.From.RelPath, result.To.RelPath)
	return nil
}

func runProjectDraftWikiPromote(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki promote", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	targetWiki := fs.String("target-wiki", "", "target upstream LLM Wiki topic; overrides draft frontmatter target_wiki")
	targetType := fs.String("target-type", "", "target upstream LLM Wiki raw type; defaults to draft target_type or notes")
	llmWikiConfig := fs.String("llm-wiki-config", "", "llm-wiki config path; defaults to ~/.config/llm-wiki/config.json")
	confirm := fs.Bool("confirm", false, "write the approved draft into the configured llm-wiki raw directory")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("exactly one draft path is required")
	}
	result, err := core.PromoteDraftWiki(core.DraftWikiPromoteRequest{
		RepoRoot:          *repo,
		Path:              fs.Arg(0),
		TargetWiki:        *targetWiki,
		TargetType:        *targetType,
		Confirm:           *confirm,
		LLMWikiConfigPath: *llmWikiConfig,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.DryRun {
		fmt.Println("draft-wiki promote dry-run; rerun with --confirm to write an llm-wiki raw note.")
	} else {
		fmt.Printf("promoted draft to llm-wiki raw note: %s\n", result.LLMWikiRawPath)
	}
	fmt.Printf("handoff: %s\n", result.HandoffCommand)
	return nil
}

func parseDraftWikiPathFlags(name string, args []string) (path, repo string, jsonOut bool, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	repoFlag := fs.String("repo", ".", "target repository path")
	jsonFlag := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return "", "", false, err
	}
	if fs.NArg() != 1 {
		return "", "", false, fmt.Errorf("exactly one draft path is required")
	}
	return fs.Arg(0), *repoFlag, *jsonFlag, nil
}
