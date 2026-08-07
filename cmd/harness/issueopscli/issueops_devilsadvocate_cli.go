package issueopscli

import (
	"flag"
	"fmt"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func runIssueOpsDevilsAdvocate(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops devils-advocate review --id ID --verdict pass|revise|stop [--finding TEXT]... [--waive --waiver-rationale TEXT] [--json]")
		return nil
	}
	if args[0] != "review" {
		return fmt.Errorf("unknown issueops devils-advocate subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops devils-advocate review", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	verdict := fs.String("verdict", "", "devil's-advocate verdict: pass|revise|stop")
	waive := fs.Bool("waive", false, "explicitly waive a stop/revise verdict")
	rationale := fs.String("waiver-rationale", "", "rationale required when --waive is set")
	jsonOut := fs.Bool("json", false, "print JSON")
	var findings repeatedFlag
	fs.Var(&findings, "finding", "surfaced problem (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := issueopscore.RecordIssueOpsDevilsAdvocateReviewWithActor(issueopscore.IssueOpsStateRoot(), *id, issueopscontract.IssueOpsDevilsAdvocateReviewRequest{
		Verdict:         *verdict,
		Findings:        findings,
		Waived:          *waive,
		WaiverRationale: *rationale,
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}
