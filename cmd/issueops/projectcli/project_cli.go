package projectcli

import (
	"flag"
	"fmt"
	projectbootstrapcontract "issueops/internal/contract/projectbootstrap"
	projectdocscontract "issueops/internal/contract/projectdocs"
	"strings"
)

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
	result, err := bootstrapProjectDocs(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: *repo, Write: *write && !*dryRun, Sync: *sync})
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
	result, err := RouteProjectDocs(*repo, "general")
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
	result, err := RouteProjectDocs(*repo, *task)
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

func runProjectAppend(args []string) error {
	fs := flag.NewFlagSet("project append", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	kind := fs.String("kind", "", "append kind: caution or adr")
	title := fs.String("title", "", "append entry title")
	summary := fs.String("summary", "", "brief summary")
	context := fs.String("context", "", "problem or decision context")
	resolution := fs.String("resolution", "", "resolution for caution/problem records")
	decision := fs.String("decision", "", "decision for ADR records")
	consequences := fs.String("consequences", "", "consequences for ADR records")
	source := fs.String("source", "cli", "append source")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := AppendProjectDocsEntry(projectdocscontract.ProjectDocsAppendRequest{
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
	fmt.Printf("appended %s in %s (%d bytes)\n", result.RecordKind, result.RelPath, result.BytesAppended)
	return nil
}
