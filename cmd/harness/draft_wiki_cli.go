package main

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
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
  agent-harness project draft-wiki suggest [--repo PATH] --input PATH [--title TITLE] [--target-wiki NAME] [--target-type notes] [--agy-command agy] [--agy-model MODEL] [--dry-run] [--json]
  agent-harness project draft-wiki approve [--repo PATH] [--json] PATH
  agent-harness project draft-wiki reject [--repo PATH] [--json] PATH
  agent-harness project draft-wiki promote [--repo PATH] [--target-wiki NAME] [--target-type notes] [--confirm] [--json] PATH
  agent-harness project draft-wiki prune [--repo PATH] [--all] [--keep N] [--json]
`)
}

func runProjectDraftWikiInit(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki init", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	dryRun := fs.Bool("dry-run", false, "show draft-wiki staging plan without writing")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	result, err := core.InitDraftWiki(core.DraftWikiInitRequest{RepoRoot: *repo, Write: !*dryRun})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	action := "initialized"
	if result.DryRun {
		action = "would initialize"
	}
	fmt.Printf("draft-wiki %s %d files in %s\n", action, len(result.Files), result.DraftDir)
	for _, file := range result.Files {
		fmt.Printf("- %s %s\n", file.Action, file.RelPath)
	}
	return nil
}

func runProjectDraftWikiList(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki list", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	result, err := core.ListDraftWiki(core.DraftWikiListRequest{RepoRoot: *repo})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("draft-wiki: %d drafts in %s\n", len(result.Drafts), result.DraftDir)
	for _, draft := range result.Drafts {
		target := draft.TargetWiki
		if target == "" {
			target = "-"
		}
		fmt.Printf("- %s [%s] target=%s title=%s\n", draft.RelPath, draft.Status, target, draft.Title)
	}
	return nil
}

func runProjectDraftWikiSuggest(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki suggest", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	input := fs.String("input", "", "source text/markdown file to summarize into a draft")
	title := fs.String("title", "", "draft title hint")
	targetWiki := fs.String("target-wiki", "", "target upstream LLM Wiki topic")
	targetType := fs.String("target-type", "notes", "target upstream LLM Wiki raw type")
	agyCommand := fs.String("agy-command", "agy", "Antigravity CLI executable")
	agyModel := fs.String("agy-model", "", "required agy settings.json model label; defaults to current settings model")
	agySettings := fs.String("agy-settings", "", "agy settings.json path; defaults to ~/.gemini/antigravity-cli/settings.json")
	dryRun := fs.Bool("dry-run", false, "validate inputs and model selection without invoking agy")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" && fs.NArg() == 1 {
		*input = fs.Arg(0)
	}
	result, err := core.SuggestDraftWiki(core.DraftWikiSuggestRequest{
		RepoRoot:        *repo,
		InputPath:       *input,
		Title:           *title,
		TargetWiki:      *targetWiki,
		TargetType:      *targetType,
		AgyCommand:      *agyCommand,
		AgyModel:        *agyModel,
		AgySettingsPath: *agySettings,
		Write:           !*dryRun,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.DryRun {
		fmt.Printf("draft-wiki suggest dry-run: %s model=%q prompt_bytes=%d\n", result.Command, result.AgyModel, result.PromptBytes)
		return nil
	}
	fmt.Printf("suggested draft: %s\n", result.Draft.RelPath)
	return nil
}

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

func runProjectDraftWikiPrune(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki prune", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	all := fs.Bool("all", false, "prune every project draft-wiki queue under the harness state dir")
	keep := fs.Int("keep", 0, "number of newest queue events to keep")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		if fs.NArg() != 1 || *all {
			return fmt.Errorf("unexpected prune argument(s)")
		}
		*repo = fs.Arg(0)
	}
	if *keep < 0 {
		return fmt.Errorf("--keep must be >= 0")
	}
	if *all {
		result, err := core.PruneAllDraftWikiQueues("", *keep)
		if err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("pruned %d draft-wiki queue event(s); before=%d after=%d keep=%d queues=%d\n", result.Pruned, result.Before, result.After, result.Keep, len(result.Queues))
		for _, queue := range result.Queues {
			fmt.Printf("- %s before=%d after=%d pruned=%d\n", queue.Path, queue.Before, queue.After, queue.Pruned)
		}
		return nil
	}
	result, err := core.PruneDraftWikiQueue(*repo, *keep)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("pruned %s: before=%d after=%d pruned=%d keep=%d\n", result.Path, result.Before, result.After, result.Pruned, result.Keep)
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
