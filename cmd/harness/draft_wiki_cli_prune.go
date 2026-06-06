package main

import (
	"flag"
	"fmt"

	"agent-harness/internal/core"
)

func runProjectDraftWikiPrune(args []string) error {
	fs := flag.NewFlagSet("project draft-wiki prune", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	all := fs.Bool("all", false, "prune every project draft-wiki queue under the harness state dir")
	keep := fs.Int("keep", 0, "number of newest queue events to keep")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		if fs.NArg() != 1 || *all {
			return fmt.Errorf("unexpected prune argument(s)")
		}
		*repo = fs.Arg(0)
	}
	if *keep < 0 {
		return fmt.Errorf("--keep must be >= 0")
	}
	if *all {
		result, err := core.PruneAllDraftWikiQueues("", *keep)
		if err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("pruned %d draft-wiki queue event(s); before=%d after=%d keep=%d queues=%d\n", result.Pruned, result.Before, result.After, result.Keep, len(result.Queues))
		for _, queue := range result.Queues {
			fmt.Printf("- %s before=%d after=%d pruned=%d\n", queue.Path, queue.Before, queue.After, queue.Pruned)
		}
		return nil
	}
	result, err := core.PruneDraftWikiQueue(*repo, *keep)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("pruned %s: before=%d after=%d pruned=%d keep=%d\n", result.Path, result.Before, result.After, result.Pruned, result.Keep)
	return nil
}
