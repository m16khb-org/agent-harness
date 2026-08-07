package basiccli

import (
	"flag"
	"fmt"

	"agent-harness/internal/adapter/core"
)

func runDocs(args []string) error {
	return runDocsWithRoot(args, deps.HarnessRoot())
}

func runDocsWithRoot(args []string, root string) error {
	if len(args) > 0 && args[0] == "index" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("docs", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result := core.DocsIndex(root, deps.Version)
	if *jsonOut {
		return printJSON(result)
	}
	for _, doc := range result.Docs {
		if doc.Title == "" {
			fmt.Println(doc.RelPath)
			continue
		}
		fmt.Printf("%s — %s\n", doc.RelPath, doc.Title)
	}
	return nil
}
