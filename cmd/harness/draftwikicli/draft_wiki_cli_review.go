package draftwikicli

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
	confirm := fs.Bool("confirm", false, "move the approved draft into the repo-local exported directory")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("exactly one draft path is required")
	}
	result, err := core.PromoteDraftWiki(core.DraftWikiPromoteRequest{
		RepoRoot: *repo,
		Path:     fs.Arg(0),
		Confirm:  *confirm,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.DryRun {
		fmt.Printf("draft-wiki promote dry-run; rerun with --confirm to export locally: %s\n", result.ExportRel)
	} else {
		fmt.Printf("exported draft: %s -> %s\n", result.From.RelPath, result.ExportRel)
	}
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
