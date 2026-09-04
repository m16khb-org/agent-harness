package issueopscli

import (
	"flag"
	"fmt"
	"time"
)

func runIssueOpsPrune(args []string) error {
	fs := flag.NewFlagSet("issueops prune", flag.ContinueOnError)
	maxAge := fs.String("max-age", "720h", "prune done cycles older than this Go duration (720h = 30 days)")
	confirm := fs.Bool("confirm", false, "delete the selected cycles; omit to preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: issueops prune [--max-age DURATION] [--confirm] [--json]")
		fs.PrintDefaults()
	}
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	age, err := time.ParseDuration(*maxAge)
	if err != nil {
		return fmt.Errorf("invalid --max-age: %w", err)
	}
	result, err := issueOpsCLIDeps.PruneIssueOps(issueOpsCLIDeps.IssueOpsStateRoot(), age, *confirm)
	if err != nil {
		if *jsonOut {
			if printErr := printJSON(result); printErr != nil {
				return printErr
			}
		} else {
			fmt.Printf(
				"incomplete: %d done cycles pruned, %d kept, %d unreadable, %d delete failures (cutoff %s)\n",
				len(result.Pruned),
				len(result.Kept),
				result.ReadErrors,
				result.DeleteErrors,
				result.Cutoff,
			)
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
	fmt.Printf(
		"%s: %d done cycles selected, %d kept, %d unreadable (cutoff %s)\n",
		mode,
		len(result.Pruned),
		len(result.Kept),
		len(result.Unreadable),
		result.Cutoff,
	)
	return nil
}
