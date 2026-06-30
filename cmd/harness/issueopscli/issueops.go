package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkcmd"
	"agent-harness/cmd/harness/issueopscli/feedbackcleanup"
	"agent-harness/cmd/harness/issueopscli/remotecmd"
	"agent-harness/cmd/harness/issueopscli/worktreecmd"
	"agent-harness/internal/adapter/provider"
	"flag"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core"
)

// issueOpsSubcommands is the dispatch registry for `issueops <subcommand>`.
// Routing is a single map lookup so adding a subcommand means adding one entry
// plus its handler, instead of growing a high-branch switch.
var issueOpsSubcommands = map[string]func([]string) error{
	"start":          runIssueOpsStart,
	"status":         runIssueOpsStatus,
	"intent":         runIssueOpsIntent,
	"plan-prep":      runIssueOpsPlanPrep,
	"design":         runIssueOpsDesign,
	"compatibility":  runIssueOpsCompatibility,
	"domain-review":  runIssueOpsDomainReview,
	"ai-slop-clean":  runIssueOpsAISlopClean,
	"regress":        runIssueOpsRegress,
	"link-issue":     runIssueOpsLinkIssue,
	"link-plan":      runIssueOpsLinkPlan,
	"link-worktree":  runIssueOpsLinkWorktree,
	"link-child":     runIssueOpsLinkChild,
	"link-related":   runIssueOpsLinkRelated,
	"branch":         runIssueOpsBranch,
	"worktree":       runIssueOpsWorktree,
	"phase":          runIssueOpsPhase,
	"record-routing": runIssueOpsRecordRouting,
	"routing-score":  runIssueOpsRoutingScore,
	"feedback":       runIssueOpsFeedback,
	"cleanup":        runIssueOpsCleanup,
	"benchmark":      func(args []string) error { return benchmarkcmd.Run(args) },
	"remote":         func(args []string) error { return remotecmd.Run(args, issueOpsRemoteDeps()) },
	"remote-score": func(args []string) error {
		return remotecmd.Run(append([]string{"score"}, args...), issueOpsRemoteDeps())
	},
	"pr-readiness":  runIssueOpsPRReadiness,
	"force-release": runIssueOpsForceRelease,
	"resume":        runIssueOpsResume,
	"decision":      runIssueOpsDecision,
	"execution":     runIssueOpsExecution,
}

func runIssueOps(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		issueOpsUsage()
		return nil
	}
	handler, ok := issueOpsSubcommands[args[0]]
	if !ok {
		return fmt.Errorf("unknown issueops subcommand %q%s", args[0], suggestIssueOpsSubcommand(args[0]))
	}
	return handler(args[1:])
}

// issueOpsConceptHints maps IssueOps domain vocabulary — lifecycle phase names,
// decision verbs, and ledger artifact names — that agents frequently mistake
// for CLI subcommands. The skill prose uses vivid nouns (grill, split, domain)
// while the CLI uses generic verbs (phase, remote, link-related). This hint
// bridges the naming gap so a wrong guess produces actionable guidance instead
// of a bare "unknown subcommand".
var issueOpsConceptHints = map[string]string{
	"grill":     "did you mean `issueops phase --to grill`? (grill is a lifecycle phase, not a subcommand)",
	"problem":   "did you mean `issueops phase --to problem`? (problem is a lifecycle phase, not a subcommand)",
	"implement": "did you mean `issueops phase --to implement`? (implement is a lifecycle phase, not a subcommand)",
	"split":     "did you mean `issueops remote create-child` or `issueops link-related --type splits-from`? (split is a breakdown decision, not a subcommand)",
}

// suggestIssueOpsSubcommand returns a suggestion suffix for an unknown
// subcommand: a concept hint for known phase/decision words, else a prefix
// match against the real subcommand registry. It returns "" when no useful
// suggestion exists.
func suggestIssueOpsSubcommand(input string) string {
	if hint, ok := issueOpsConceptHints[input]; ok {
		return "; " + hint
	}
	var matches []string
	for name := range issueOpsSubcommands {
		if strings.HasPrefix(name, input) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 1 {
		return fmt.Sprintf("; did you mean `%s`?", matches[0])
	}
	return ""
}

func issueOpsRemoteDeps() remotecmd.Deps {
	return remotecmd.Deps{
		PrintJSON:   printJSON,
		PrintResult: printIssueOpsResult,
		PrintError:  printIssueOpsErrorJSON,
		VerifyLive:  verifyIssueOpsRemoteArtifactLive,
	}
}

func runIssueOpsWorktree(args []string) error {
	return worktreecmd.Run(args, issueOpsWorktreeDeps())
}

func prepareIssueOpsWorktreeTools(record core.IssueOpsRecord) (worktreecmd.PrepareResult, error) {
	return worktreecmd.PrepareWorktreeTools(record)
}

func issueOpsWorktreeDeps() worktreecmd.Deps {
	return worktreecmd.Deps{
		ParseFlags: parseIssueOpsFlags,
		PrintJSON:  printJSON,
		PrintError: printIssueOpsErrorJSON,
	}
}

func parseIssueOpsFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func runIssueOpsFeedback(args []string) error {
	if len(args) > 0 && args[0] == "resolve" {
		return runIssueOpsFeedbackResolve(args[1:])
	}
	return feedbackcleanup.RunFeedback(args, issueOpsFeedbackCleanupDeps())
}

func runIssueOpsCleanup(args []string) error {
	if len(args) > 0 && args[0] == "stale" {
		return runIssueOpsCleanupStale(args[1:])
	}
	return feedbackcleanup.RunCleanup(args, issueOpsFeedbackCleanupDeps())
}

func runIssueOpsCleanupStale(args []string) error {
	fs := flag.NewFlagSet("issueops cleanup stale", flag.ContinueOnError)
	repo := fs.String("repo", "", "source repository path")
	maxAgeDays := fs.Int("max-age", 14, "age in days after which an idle non-done cycle is flagged needs-review")
	apply := fs.Bool("apply", false, "force-release confirmed-stale and likely-done cycles (default: report only)")
	pruneDone := fs.String("prune-done", "720h", "prune done cycles older than this duration (e.g. 720h for 30 days); only with --apply")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	pruneDoneAge, err := time.ParseDuration(*pruneDone)
	if err != nil {
		return fmt.Errorf("invalid --prune-done duration %q: %w", *pruneDone, err)
	}
	if pruneDoneAge < 0 {
		return fmt.Errorf("--prune-done must be non-negative, got %s", *pruneDone)
	}
	result := core.ScanStaleIssueOpsCycles(core.IssueOpsStaleScanRequest{
		Repo:         *repo,
		MaxAge:       time.Duration(*maxAgeDays) * 24 * time.Hour,
		Apply:        *apply,
		PruneDoneAge: pruneDoneAge,
	})
	if *jsonOut {
		return printJSON(result)
	}
	if !result.OK {
		return fmt.Errorf("issueops: %s", strings.Join(result.Errors, "; "))
	}
	if len(result.Findings) == 0 && result.PrunedDone == 0 {
		fmt.Printf("No stale IssueOps cycles for %s\n", result.Repo)
		return nil
	}
	for _, f := range result.Findings {
		fmt.Printf("- %s [%s] %s (%s) worktree=%s releasable=%t\n",
			f.ID, f.Phase, f.Category, strings.Join(f.Reasons, ","), f.WorktreePath, f.Releasable)
	}
	if result.Applied {
		fmt.Printf("Released %d cycle(s): %s\n", len(result.Released), strings.Join(result.Released, ", "))
	} else {
		fmt.Println("Dry run (report only). Re-run with --apply to force-release confirmed-stale/likely-done cycles.")
	}
	if result.PrunedDone > 0 {
		fmt.Printf("Pruned %d done cycle(s)\n", result.PrunedDone)
	}
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %s\n", strings.Join(result.Errors, "; "))
	}
	return nil
}

func issueOpsFeedbackCleanupDeps() feedbackcleanup.Deps {
	return feedbackcleanup.Deps{
		ParseFlags:   parseIssueOpsFlags,
		PrintResult:  printIssueOpsResult,
		PrintJSON:    printJSON,
		PrintError:   printIssueOpsErrorJSON,
		VerifyMerged: verifyIssueOpsRemoteArtifactMergedLive,
		Provider:     provider.Resolve,
	}
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	return feedbackcleanup.CleanupMerged(id, requested, issueOpsFeedbackCleanupDeps())
}
