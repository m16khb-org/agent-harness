package issueopscli

import (
	"flag"
	"fmt"

	"agent-harness/internal/core"
)

func runIssueOpsPlanPrep(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops plan-prep record --id ID [--decisions-evidence TEXT | --decisions-waive REASON] [--related-score-ref TEXT | --related-waive REASON] [--web-research-evidence TEXT | --web-research-waive REASON] [--codebase-survey-evidence TEXT | --codebase-survey-waive REASON] [--json]")
		return nil
	}
	if args[0] != "record" {
		return fmt.Errorf("unknown issueops plan-prep subcommand")
	}
	fs := flag.NewFlagSet("issueops plan-prep record", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	var decisionsEvidence repeatedFlag
	var relatedScore repeatedFlag
	var webResearch repeatedFlag
	var codebaseSurvey repeatedFlag
	fs.Var(&decisionsEvidence, "decisions-evidence", "prior decision evidence such as an ADR or decision link; repeatable")
	fs.Var(&relatedScore, "related-score-ref", "remote score result summary (selected/rejected candidates, threshold); repeatable")
	fs.Var(&webResearch, "web-research-evidence", "web research evidence such as a research file path or source; repeatable")
	fs.Var(&codebaseSurvey, "codebase-survey-evidence", "codebase survey evidence: tools used and the touched symbols/files/call paths found (rg, CodeGraph, LSP); repeatable")
	decisionsWaive := fs.String("decisions-waive", "", "reason prior-decision lookup is unnecessary")
	relatedWaive := fs.String("related-waive", "", "reason related-issue scoring is unnecessary")
	webWaive := fs.String("web-research-waive", "", "reason web research is unnecessary")
	surveyWaive := fs.String("codebase-survey-waive", "", "reason a codebase survey is unnecessary (e.g. net-new files only)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := core.RecordIssueOpsPlanPrepWithActor(core.IssueOpsStateRoot(), *id, core.IssueOpsPlanPrepRequest{
		PriorDecisions: core.IssueOpsPlanPrepItemRequest{Evidence: []string(decisionsEvidence), WaiveReason: *decisionsWaive},
		RelatedIssues:  core.IssueOpsPlanPrepItemRequest{Evidence: []string(relatedScore), WaiveReason: *relatedWaive},
		WebResearch:    core.IssueOpsPlanPrepItemRequest{Evidence: []string(webResearch), WaiveReason: *webWaive},
		CodebaseSurvey: core.IssueOpsPlanPrepItemRequest{Evidence: []string(codebaseSurvey), WaiveReason: *surveyWaive},
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}
