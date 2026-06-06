package main

import (
	"flag"
	"fmt"
)

func runSelfVerifyCompare(args []string) error {
	fs := flag.NewFlagSet("self-verify compare", flag.ContinueOnError)
	baselineKey := fs.String("baseline-key", "", "state key containing the baseline self-verification summary snapshot")
	candidateKey := fs.String("candidate-key", "", "state key containing the candidate self-verification summary snapshot")
	maxElapsedRegressionPct := fs.Float64("max-elapsed-regression-pct", 20, "allowed elapsed_ms increase percentage before regression")
	failOnRegression := fs.Bool("fail-on-regression", false, "return non-zero when comparison reports a regression")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := compareSelfAugmentSummaries(*baselineKey, *candidateKey, *maxElapsedRegressionPct)
	if err != nil {
		return err
	}
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		status := "ok"
		if result.Regressed {
			status = "regressed"
		}
		fmt.Printf("self-verify compare %s: elapsed_delta=%dms failed_steps_delta=%d\n", status, result.ElapsedDeltaMS, result.FailedStepsDelta)
		for _, regression := range result.Regressions {
			fmt.Println("- " + regression)
		}
	}
	if *failOnRegression && result.Regressed {
		return fmt.Errorf("self-verification summary regression detected")
	}
	return nil
}

func runSelfVerifyHistory(args []string) error {
	fs := flag.NewFlagSet("self-verify history", flag.ContinueOnError)
	prefix := fs.String("prefix", "self-verify", "state key prefix to scan; empty string scans all keys")
	limit := fs.Int("limit", 20, "maximum entries to return; 0 returns all")
	retentionLimit := fs.Int("retention-limit", 0, "maximum matching summaries to retain by newest-first ordering; 0 disables retention planning")
	pruneRetention := fs.Bool("prune-retention", false, "delete retention candidates; dry-run unless --confirm is also set")
	confirm := fs.Bool("confirm", false, "confirm deletion when used with --prune-retention")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := selfAugmentHistory(*prefix, *limit, selfAugmentHistoryRetentionOptions{
		Limit:          *retentionLimit,
		PruneRequested: *pruneRetention,
		Confirm:        *confirm,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("self-verify history: %d/%d entries from %s (prefix=%q)\n", result.Returned, result.TotalMatches, result.StateDir, result.Prefix)
	for _, entry := range result.Entries {
		status := "fail"
		if entry.OK {
			status = "ok"
		}
		fmt.Printf("- %s %s iterations=%d elapsed=%dms generated_at=%s\n", entry.Key, status, entry.Iterations, entry.ElapsedMS, entry.GeneratedAt)
	}
	if len(result.Skipped) > 0 {
		fmt.Printf("skipped %d non-summary records\n", len(result.Skipped))
	}
	if result.Retention != nil {
		retention := result.Retention
		action := "planned"
		if retention.PruneRequested && !retention.Confirm {
			action = "would delete"
		}
		if retention.PruneRequested && retention.Confirm {
			action = "deleted"
		}
		fmt.Printf("retention: retain=%d candidates=%d %s=%d\n", retention.Limit, len(retention.CandidateKeys), action, len(retention.DeletedKeys))
	}
	return nil
}
