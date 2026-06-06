package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/core"
)

func runProject(args []string) error {
	if len(args) == 0 {
		projectUsage()
		return fmt.Errorf("missing project subcommand")
	}
	switch args[0] {
	case "bootstrap":
		return runProjectBootstrap(args[1:])
	case "docs":
		return runProjectDocs(args[1:])
	case "route-docs":
		return runProjectRouteDocs(args[1:])
	case "record":
		return runProjectRecord(args[1:])
	case "draft-wiki":
		return runProjectDraftWiki(args[1:])
	case "commit-suggest":
		return runProjectCommitSuggest(args[1:])
	case "lint-diagnose":
		return runProjectLintDiagnose(args[1:])
	default:
		projectUsage()
		return fmt.Errorf("unknown project subcommand %q", args[0])
	}
}

func projectUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness project bootstrap [--repo PATH] [--sync] [--dry-run] [--json]
  agent-harness project docs [--repo PATH] [--json]
  agent-harness project route-docs [--repo PATH] [--task TEXT] [--json]
  agent-harness project record --kind caution|adr --title TEXT --summary TEXT [--repo PATH] [--json]
  agent-harness project draft-wiki init|list|suggest|approve|reject|promote|prune ...
  agent-harness project commit-suggest [--repo PATH] [--staged] [--agy-command CMD] [--json]
  agent-harness project lint-diagnose [--repo PATH] [--agy-command CMD] [--json] -- <command_to_run...>
`)
}

func runProjectBootstrap(args []string) error {
	fs := flag.NewFlagSet("project bootstrap", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	sync := fs.Bool("sync", false, "refresh existing project docs as well as creating missing files")
	dryRun := fs.Bool("dry-run", false, "show project docs plan without writing")
	write := fs.Bool("write", true, "compatibility alias; use --dry-run for planning")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	result, err := core.BootstrapProjectDocs(core.ProjectDocsBootstrapRequest{RepoRoot: *repo, Write: *write && !*dryRun, Sync: *sync})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	action := "would update"
	if result.Write {
		action = "updated"
	}
	fmt.Printf("project docs %s %d files in %s\n", action, len(result.Files), result.RepoRoot)
	for _, file := range result.Files {
		fmt.Printf("- %s %s\n", file.Action, file.RelPath)
	}
	stateAction := "planned"
	if result.LifecycleState.Exists && result.LifecycleState.NamespaceValid {
		stateAction = "initialized"
	}
	fmt.Printf("lifecycle state: %s (%s)\n", result.LifecycleState.ProjectStateDir, stateAction)
	return nil
}

func runProjectDocs(args []string) error {
	fs := flag.NewFlagSet("project docs", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	result, err := core.RouteProjectDocs(*repo, "general")
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	for _, doc := range result.Docs {
		fmt.Printf("%s — %s\n", doc.RelPath, doc.Reason)
	}
	return nil
}

func runProjectRouteDocs(args []string) error {
	fs := flag.NewFlagSet("project route-docs", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	task := fs.String("task", "general", "task description such as commit, test, architecture, dependency, deploy")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*task = strings.Join(fs.Args(), " ")
	}
	result, err := core.RouteProjectDocs(*repo, *task)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	for _, doc := range result.Docs {
		status := "missing"
		if doc.Exists {
			status = "exists"
		}
		fmt.Printf("%s [%s] — %s\n", doc.RelPath, status, doc.Reason)
	}
	return nil
}

func runProjectRecord(args []string) error {
	fs := flag.NewFlagSet("project record", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	kind := fs.String("kind", "", "record kind: caution or adr")
	title := fs.String("title", "", "record title")
	summary := fs.String("summary", "", "brief summary")
	context := fs.String("context", "", "problem or decision context")
	resolution := fs.String("resolution", "", "resolution for caution/problem records")
	decision := fs.String("decision", "", "decision for ADR records")
	consequences := fs.String("consequences", "", "consequences for ADR records")
	source := fs.String("source", "cli", "record source")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.AppendProjectDocsRecord(core.ProjectDocsRecordRequest{
		RepoRoot:     *repo,
		Kind:         *kind,
		Title:        *title,
		Summary:      *summary,
		Context:      *context,
		Resolution:   *resolution,
		Decision:     *decision,
		Consequences: *consequences,
		Source:       *source,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("recorded %s in %s (%d bytes)\n", result.RecordKind, result.RelPath, result.BytesAppended)
	return nil
}
