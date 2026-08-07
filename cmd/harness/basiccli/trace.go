package basiccli

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/adapter/core"
)

func runTrace(args []string) error {
	if len(args) == 0 {
		traceUsage()
		return fmt.Errorf("missing trace subcommand")
	}
	switch args[0] {
	case "analyze":
		return runTraceAnalyze(args[1:])
	default:
		traceUsage()
		return fmt.Errorf("unknown trace subcommand %q", args[0])
	}
}

func traceUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness trace analyze --input <jsonl|state-key> [--json]
`)
}

func runTraceAnalyze(args []string) error {
	fs := flag.NewFlagSet("trace analyze", flag.ContinueOnError)
	input := fs.String("input", "", "trace input path, '-' for stdin, or harness state key")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" && fs.NArg() > 0 {
		*input = fs.Arg(0)
	}
	result, err := core.TraceAnalyze(core.TraceAnalyzeRequest{Input: *input})
	if err != nil {
		if *jsonOut {
			_ = printJSON(result)
		} else {
			traceUsage()
		}
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("trace analysis: %d finding(s) from %s\n", result.FindingCount, result.InputSource)
	for _, finding := range result.Findings {
		fmt.Printf("- %s: %s\n", finding.FailureClass, finding.RecurringPattern)
		fmt.Printf("  knob: %s\n", finding.ProposedKnob)
		fmt.Printf("  verify: %s\n", finding.VerificationCommand)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}
