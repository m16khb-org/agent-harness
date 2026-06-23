package draftwikicli

import (
	"flag"
	"fmt"

	"agent-harness/internal/core"
)

func runProjectDraftWikiSuggest(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki suggest", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	input := fs.String("input", "", "source text/markdown file to summarize into a draft")
	title := fs.String("title", "", "draft title hint")
	targetWiki := fs.String("target-wiki", "", "target upstream LLM Wiki topic")
	targetType := fs.String("target-type", "notes", "target upstream LLM Wiki raw type")
	model := fs.String("model", "", "Z.AI model; defaults to glm-5-turbo")
	dryRun := fs.Bool("dry-run", false, "validate inputs without invoking the external LLM")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" && fs.NArg() == 1 {
		*input = fs.Arg(0)
	}
	result, err := core.SuggestDraftWiki(core.DraftWikiSuggestRequest{
		RepoRoot:   *repo,
		InputPath:  *input,
		Title:      *title,
		TargetWiki: *targetWiki,
		TargetType: *targetType,
		Model:      *model,
		Write:      !*dryRun,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.DryRun {
		fmt.Printf("draft-wiki suggest dry-run: %s model=%q prompt_bytes=%d\n", result.Command, result.Model, result.PromptBytes)
		return nil
	}
	fmt.Printf("suggested draft: %s\n", result.Draft.RelPath)
	return nil
}
