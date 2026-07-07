package issueopscli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

func runIssueOpsExecution(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops execution decide --id ID --auto TEXT --hook-block TEXT --human-gate TEXT --subagent-use none|planned [--subagent-rationale TEXT] [--subagent-plan-file PATH] [--json]")
		return nil
	}
	if args[0] != "decide" {
		return fmt.Errorf("unknown issueops execution subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops execution decide", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	subagentUse := fs.String("subagent-use", "none", "sub-agent use: none or planned")
	subagentRationale := fs.String("subagent-rationale", "", "why sub-agents are not used, or the top-level planned-use rationale")
	subagentPlanFile := fs.String("subagent-plan-file", "", "strict JSON file containing an array of sub-agent plans")
	jsonOut := fs.Bool("json", false, "print JSON")
	var autoProceed repeatedFlag
	var hookBlocked repeatedFlag
	var humanGates repeatedFlag
	fs.Var(&autoProceed, "auto", "auto-proceed condition (repeatable)")
	fs.Var(&hookBlocked, "hook-block", "work hooks must not perform (repeatable)")
	fs.Var(&humanGates, "human-gate", "human-in-the-loop condition (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	plans, err := readIssueOpsSubagentPlanFile(*subagentPlanFile)
	if err != nil {
		if *jsonOut {
			_ = printIssueOpsErrorJSON(err)
		}
		return err
	}
	record, err := core.RecordIssueOpsExecutionDecision(core.IssueOpsStateRoot(), *id, core.IssueOpsExecutionDecisionRecordRequest{
		AutoProceed:       autoProceed,
		HookBlocked:       hookBlocked,
		HumanGates:        humanGates,
		SubagentUse:       *subagentUse,
		SubagentRationale: *subagentRationale,
		SubagentPlans:     plans,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

func readIssueOpsSubagentPlanFile(path string) ([]core.IssueOpsSubAgentPlan, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var plans []core.IssueOpsSubAgentPlan
	if err := decoder.Decode(&plans); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("subagent plan file must contain one JSON array")
	}
	return plans, nil
}
