package draftwikicli

import (
	draftwiki "agent-harness/internal/adapter/draftwiki"
	"flag"
	"fmt"
)

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
	result, err := draftwiki.InitDraftWiki(draftwiki.DraftWikiInitRequest{RepoRoot: *repo, Write: !*dryRun})
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
	result, err := draftwiki.ListDraftWiki(draftwiki.DraftWikiListRequest{RepoRoot: *repo})
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
