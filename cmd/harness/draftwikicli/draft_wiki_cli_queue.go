package draftwikicli

import (
	draftwiki "agent-harness/internal/adapter/draftwiki"
	"flag"
	"fmt"
)

func runProjectDraftWikiQueue(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki queue", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	input := fs.String("input", "", "source text/markdown file to queue after main-agent judgement")
	material := fs.String("material", "", "source material to queue after main-agent judgement")
	stdinFlag := fs.Bool("stdin", false, "read source material from stdin")
	targetWiki := fs.String("target-wiki", "", "target wiki topic")
	targetType := fs.String("target-type", "notes", "target wiki entry type")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" && *material == "" && !*stdinFlag && fs.NArg() == 1 {
		*input = fs.Arg(0)
	}
	sourceMaterial, err := draftWikiQueueMaterial(*repo, *input, *material, *stdinFlag)
	if err != nil {
		return err
	}
	result, err := draftwiki.AppendDraftWikiQueueEvent(draftwiki.DraftWikiQueueAppendRequest{
		RepoRoot:       *repo,
		SourceMaterial: sourceMaterial,
		TargetWiki:     *targetWiki,
		TargetType:     *targetType,
		Source:         "main-agent",
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("queued draft-wiki material: %s\n", result.Event.ID)
	return nil
}

func draftWikiQueueMaterial(repo, input, material string, stdinFlag bool) (string, error) {
	return QueueMaterial(repo, input, material, stdinFlag)
}
