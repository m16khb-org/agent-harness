package issueopscli

import (
	"flag"
	"fmt"
	"time"

	"agent-harness/internal/adapter/core"
)

func runIssueOpsPrune(args []string) error {
	fs := flag.NewFlagSet("issueops prune", flag.ContinueOnError)
	maxAge := fs.String("max-age", "720h", "prune done cycles older than this Go duration (720h = 30 days)")
	confirm := fs.Bool("confirm", false, "delete the selected cycles; omit to preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: agent-harness issueops prune [--max-age DURATION] [--confirm] [--json]")
		fs.PrintDefaults()
	}
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	age, err := time.ParseDuration(*maxAge)
	if err != nil {
		return fmt.Errorf("invalid --max-age: %w", err)
	}
	result, err := core.PruneIssueOps(core.IssueOpsStateRoot(), age, *confirm)
	if err != nil {
		if *jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	mode := "preview"
	if *confirm {
		mode = "pruned"
	}
	fmt.Printf("%s: %d done cycles selected, %d kept (cutoff %s)\n", mode, len(result.Pruned), len(result.Kept), result.Cutoff)
	return nil
}
