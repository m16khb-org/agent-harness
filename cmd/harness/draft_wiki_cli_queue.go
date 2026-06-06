package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core"
)

func runProjectDraftWikiQueue(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki queue", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	input := fs.String("input", "", "source text/markdown file to queue after main-agent judgement")
	material := fs.String("material", "", "source material to queue after main-agent judgement")
	stdinFlag := fs.Bool("stdin", false, "read source material from stdin")
	targetWiki := fs.String("target-wiki", "", "target upstream LLM Wiki topic")
	targetType := fs.String("target-type", "notes", "target upstream LLM Wiki raw type")
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
	result, err := core.AppendDraftWikiQueueEvent(core.DraftWikiQueueAppendRequest{
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
	count := 0
	if strings.TrimSpace(input) != "" {
		count++
	}
	if strings.TrimSpace(material) != "" {
		count++
	}
	if stdinFlag {
		count++
	}
	if count != 1 {
		return "", fmt.Errorf("exactly one of --input, --material, or --stdin is required")
	}
	if strings.TrimSpace(material) != "" {
		return material, nil
	}
	if stdinFlag {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, filepath.FromSlash(path))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
