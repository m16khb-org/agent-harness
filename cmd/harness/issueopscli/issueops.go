package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkcmd"
	"agent-harness/cmd/harness/issueopscli/feedbackcleanup"
	"agent-harness/cmd/harness/issueopscli/remotecmd"
	"agent-harness/internal/adapter/provider"
	"flag"
	"fmt"
	"strings"
)

// issueOpsSubcommands is the dispatch registry for `issueops <subcommand>`.
// Routing is a single map lookup so adding a subcommand means adding one entry
// plus its handler, instead of growing a high-branch switch.
var issueOpsSubcommands = map[string]func([]string) error{
	"start":           runIssueOpsStart,
	"status":          runIssueOpsStatus,
	"intent":          runIssueOpsIntent,
	"plan-prep":       runIssueOpsPlanPrep,
	"design":          runIssueOpsDesign,
	"compatibility":   runIssueOpsCompatibility,
	"devils-advocate": runIssueOpsDevilsAdvocate,
	"domain-review":   runIssueOpsDomainReview,
	"ai-slop-clean":   runIssueOpsAISlopClean,
	"regress":         runIssueOpsRegress,
	"link-issue":      runIssueOpsLinkIssue,
	"link-plan":       runIssueOpsLinkPlan,
	"link-worktree":   runIssueOpsLinkWorktree,
	"link-child":      runIssueOpsLinkChild,
	"link-related":    runIssueOpsLinkRelated,
	"child":           runIssueOpsChild,
	"branch":          runIssueOpsBranch,
	"phase":           runIssueOpsPhase,
	"record-routing":  runIssueOpsRecordRouting,
	"routing-score":   runIssueOpsRoutingScore,
	"feedback":        runIssueOpsFeedback,
	"cleanup":         runIssueOpsCleanup,
	"benchmark":       func(args []string) error { return benchmarkcmd.Run(args) },
	"remote":          func(args []string) error { return remotecmd.Run(args, issueOpsRemoteDeps()) },
	"remote-score": func(args []string) error {
		return remotecmd.Run(append([]string{"score"}, args...), issueOpsRemoteDeps())
	},
	"pr-readiness": runIssueOpsPRReadiness,
	"decision":     runIssueOpsDecision,
	"execution":    runIssueOpsExecution,
	"reset-legacy": runIssueOpsResetLegacy,
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
	return feedbackcleanup.RunCleanup(args, issueOpsFeedbackCleanupDeps())
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
