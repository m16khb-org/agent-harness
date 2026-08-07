package draftwikicli

import (
	draftwiki "agent-harness/internal/adapter/draftwiki"
	"flag"
	"fmt"
)

func runProjectDraftWikiSuggest(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki suggest", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	input := fs.String("input", "", "source text/markdown file to summarize into a draft")
	title := fs.String("title", "", "draft title hint")
	targetWiki := fs.String("target-wiki", "", "target wiki topic")
	targetType := fs.String("target-type", "notes", "target raw type")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" && fs.NArg() == 1 {
		*input = fs.Arg(0)
	}
	result, err := draftwiki.SuggestDraftWiki(draftwiki.DraftWikiSuggestRequest{
		RepoRoot:   *repo,
		InputPath:  *input,
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
	fmt.Println(result.Prompt)
	return nil
}
