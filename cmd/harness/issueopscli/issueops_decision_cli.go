package issueopscli

import (
	"flag"
	"fmt"
	"strings"

	"agent-harness/internal/core"
)

func runIssueOpsDecision(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops decision add --id ID --title TEXT --body TEXT --kind product|architecture|implementation|test|review|scope|follow-up [--rationale TEXT] [--alternative TEXT]... [--affected-link URL]... [--affected-artifact issue|plan|test|implementation|review|pr_mr|follow-up]... [--json]")
		return nil
	}
	if args[0] != "add" {
		return fmt.Errorf("unknown issueops decision subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops decision add", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	title := fs.String("title", "", "decision title")
	body := fs.String("body", "", "decision body")
	kind := fs.String("kind", "", "decision kind: product, architecture, implementation, test, review, scope, follow-up")
	rationale := fs.String("rationale", "", "decision rationale")
	jsonOut := fs.Bool("json", false, "print JSON")
	var alternatives sliceFlag
	var affectedLinks sliceFlag
	var affectedArtifacts sliceFlag
	fs.Var(&alternatives, "alternative", "alternatives considered (repeatable)")
	fs.Var(&affectedLinks, "affected-link", "affected issue link URLs (repeatable)")
	fs.Var(&affectedArtifacts, "affected-artifact", "affected artifacts: issue, plan, test, implementation, review, pr_mr, follow-up (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := core.AddIssueOpsDecision(core.IssueOpsStateRoot(), *id, core.IssueOpsDecisionRecordRequest{
		Title:              *title,
		Body:               *body,
		Kind:               *kind,
		Rationale:          *rationale,
		Alternatives:       alternatives,
		AffectedIssueLinks: affectedLinks,
		AffectedArtifacts:  affectedArtifacts,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

type sliceFlag []string

func (f *sliceFlag) String() string     { return strings.Join(*f, ", ") }
func (f *sliceFlag) Set(v string) error { *f = append(*f, v); return nil }
