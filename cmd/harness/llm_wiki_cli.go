package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"agent-harness/internal/core"
)

func runLLMWiki(args []string) error {
	if len(args) == 0 {
		llmWikiUsage()
		return fmt.Errorf("missing llm-wiki subcommand")
	}
	switch args[0] {
	case "inventory":
		return runLLMWikiInventory(args[1:])
	case "session-context":
		return runLLMWikiSessionContext(args[1:])
	case "search":
		return runLLMWikiSearch(args[1:])
	case "read":
		return runLLMWikiRead(args[1:])
	case "capture":
		return runLLMWikiCapture(args[1:])
	default:
		llmWikiUsage()
		return fmt.Errorf("unknown llm-wiki subcommand %q", args[0])
	}
}

func llmWikiUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  harness llm-wiki inventory [--root PATH] [--project PATH] [--json]
  harness llm-wiki session-context [--root PATH] [--project PATH] [--json]
  harness llm-wiki search --query TEXT [--root PATH] [--limit N] [--json]
  harness llm-wiki read --page PATH_OR_SLUG [--root PATH] [--json]
  harness llm-wiki capture --title TITLE (--content TEXT|--input FILE|--stdin) [--type session|concept|entity|summary] [--tag TAG] [--source WIKILINK] [--related WIKILINK] [--root PATH] [--project PATH] [--json]
`)
}

func runLLMWikiInventory(args []string) error {
	fs := flag.NewFlagSet("llm-wiki inventory", flag.ContinueOnError)
	root := fs.String("root", "", "llm-wiki root")
	project := fs.String("project", "", "current project path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	projectPath := *project
	if projectPath == "" {
		projectPath = resolveTarget("")
	}
	result, err := core.LLMWikiInventoryFor(*root, projectPath)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("llm-wiki root: %s\n", result.Root)
	fmt.Printf("status: %s\n", result.Status)
	fmt.Printf("markdown: %d, sources: %d, concepts: %d, entities: %d, summaries: %d, sessions: %d\n", result.Counts.MarkdownFiles, result.Counts.Sources, result.Counts.Concepts, result.Counts.Entities, result.Counts.Summaries, result.Counts.Sessions)
	for _, entry := range result.EntryPoints {
		fmt.Printf("- %s exists=%v bytes=%d\n", entry.Path, entry.Exists, entry.Bytes)
	}
	return nil
}

func runLLMWikiSessionContext(args []string) error {
	fs := flag.NewFlagSet("llm-wiki session-context", flag.ContinueOnError)
	root := fs.String("root", "", "llm-wiki root")
	project := fs.String("project", "", "current project path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	projectPath := *project
	if projectPath == "" {
		projectPath = resolveTarget("")
	}
	result, err := core.LLMWikiSessionContextFor(*root, projectPath)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Print(result.Text)
	return nil
}

func runLLMWikiSearch(args []string) error {
	fs := flag.NewFlagSet("llm-wiki search", flag.ContinueOnError)
	root := fs.String("root", "", "llm-wiki root")
	query := fs.String("query", "", "search query")
	limit := fs.Int("limit", 10, "result limit")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *query == "" && fs.NArg() > 0 {
		*query = strings.Join(fs.Args(), " ")
	}
	result, err := core.LLMWikiSearch(*root, *query, *limit)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	for _, item := range result.Results {
		fmt.Printf("%s score=%d title=%q\n", item.Path, item.Score, item.Title)
		if item.Snippet != "" {
			fmt.Printf("  %s\n", item.Snippet)
		}
	}
	return nil
}

func runLLMWikiRead(args []string) error {
	fs := flag.NewFlagSet("llm-wiki read", flag.ContinueOnError)
	root := fs.String("root", "", "llm-wiki root")
	page := fs.String("page", "", "relative path, slug, or wikilink")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *page == "" && fs.NArg() > 0 {
		*page = fs.Arg(0)
	}
	result, err := core.LLMWikiRead(*root, *page)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Print(result.Content)
	return nil
}

func runLLMWikiCapture(args []string) error {
	fs := flag.NewFlagSet("llm-wiki capture", flag.ContinueOnError)
	root := fs.String("root", "", "llm-wiki root")
	title := fs.String("title", "", "page title")
	content := fs.String("content", "", "capture content")
	input := fs.String("input", "", "read content from file")
	typ := fs.String("type", "session", "session, concept, entity, or summary")
	project := fs.String("project", "", "current project path")
	status := fs.String("status", "active", "page status")
	jsonOut := fs.Bool("json", false, "print JSON")
	var tags repeatedStringFlag
	var sources repeatedStringFlag
	var related repeatedStringFlag
	fs.Var(&tags, "tag", "tag; repeatable")
	fs.Var(&sources, "source", "source wikilink; repeatable")
	fs.Var(&related, "related", "related wikilink; repeatable")
	stdin := fs.Bool("stdin", false, "read content from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	body := *content
	if *input != "" {
		b, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		body = string(b)
	}
	if *stdin {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		body = string(b)
	}
	projectPath := *project
	if projectPath == "" {
		projectPath = resolveTarget("")
	}
	result, err := core.LLMWikiCapture(core.LLMWikiCaptureRequest{
		Root:        *root,
		Title:       *title,
		Content:     body,
		Type:        *typ,
		Tags:        tags,
		Sources:     sources,
		Related:     related,
		ProjectPath: projectPath,
		Status:      *status,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("captured %s\n", result.Path)
	return nil
}

type repeatedStringFlag []string

func (f *repeatedStringFlag) String() string { return strings.Join(*f, ",") }
func (f *repeatedStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
