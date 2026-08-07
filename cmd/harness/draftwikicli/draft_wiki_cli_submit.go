package draftwikicli

import (
	draftwiki "agent-harness/internal/adapter/draftwiki"
	"flag"
	"fmt"
)

func runProjectDraftWikiSubmit(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki submit", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	draftPath := fs.String("draft", "", "host-agent-authored Markdown draft file")
	title := fs.String("title", "", "draft title hint")
	targetWiki := fs.String("target-wiki", "", "target wiki topic")
	targetType := fs.String("target-type", "notes", "target raw type")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *draftPath == "" && fs.NArg() == 1 {
		*draftPath = fs.Arg(0)
	}
	result, err := draftwiki.SubmitDraftWiki(draftwiki.DraftWikiSubmitRequest{
		RepoRoot:   *repo,
		DraftPath:  *draftPath,
		Title:      *title,
		TargetWiki: *targetWiki,
		TargetType: *targetType,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("submitted draft: %s\n", result.Draft.RelPath)
	return nil
}
