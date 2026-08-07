package issueopscli

import (
	"flag"
	"fmt"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func runIssueOpsDomainReview(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops domain-review record --id ID --model-fit TEXT [--terminology TEXT] [--risk TEXT] [--uncertainty TEXT] [--json]")
		return nil
	}
	if args[0] != "record" {
		return fmt.Errorf("unknown issueops domain-review subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops domain-review record", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	modelFit := fs.String("model-fit", "", "how the change fits the current domain model")
	jsonOut := fs.Bool("json", false, "print JSON")
	var terminology repeatedFlag
	var risks repeatedFlag
	var uncertainties repeatedFlag
	fs.Var(&terminology, "terminology", "domain terminology note (repeatable)")
	fs.Var(&risks, "risk", "domain risk (repeatable)")
	fs.Var(&uncertainties, "uncertainty", "unresolved uncertainty (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := issueopscore.RecordIssueOpsDomainReviewWithActor(issueopscore.IssueOpsStateRoot(), *id, issueopscontract.IssueOpsDomainReviewRequest{
		Terminology:       terminology,
		ModelFit:          *modelFit,
		Risks:             risks,
		OpenUncertainties: uncertainties,
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsAISlopClean(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops ai-slop-clean record --id ID --category TEXT --verification TEXT --host HOST --session-id SESSION --cwd PATH [--agent-id ID] [--json]")
		return nil
	}
	if args[0] != "record" {
		return fmt.Errorf("unknown issueops ai-slop-clean subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops ai-slop-clean record", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	jsonOut := fs.Bool("json", false, "print JSON")
	var categories repeatedFlag
	var verification repeatedFlag
	fs.Var(&categories, "category", "cleanup category checked or cleaned (repeatable)")
	fs.Var(&verification, "verification", "verification rerun after cleanup (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := issueopscore.RecordIssueOpsAISlopCleanEvidenceWithActor(issueopscore.IssueOpsStateRoot(), *id, categories, verification, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsRegress(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Println("Usage: agent-harness issueops regress --id ID --reason TEXT [--json]  (Brooks stop -> regress to grill for re-plan)")
		return nil
	}
	fs := flag.NewFlagSet("issueops regress", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	reason := fs.String("reason", "", "the Brooks stop verdict / regression reason")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueopscore.RegressIssueOpsForReplanWithActor(issueopscore.IssueOpsStateRoot(), *id, *reason, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsFeedbackResolve(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Println("Usage: agent-harness issueops feedback resolve --id ID --index N --resolution valid-defect|question-answered|noise-dismissed [--json]")
		return nil
	}
	fs := flag.NewFlagSet("issueops feedback resolve", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	index := fs.Int("index", -1, "feedback item index (0-based)")
	resolution := fs.String("resolution", "", "resolution outcome: valid-defect, question-answered, or noise-dismissed")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueopscore.ResolveIssueOpsFeedbackWithActor(issueopscore.IssueOpsStateRoot(), *id, *index, *resolution, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}
