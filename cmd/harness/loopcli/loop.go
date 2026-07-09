package loopcli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

type repeatedFlag []string

func (r *repeatedFlag) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprint([]string(*r))
}

func (r *repeatedFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		loopUsage()
		return nil
	}
	switch args[0] {
	case "start":
		return runStart(args[1:])
	case "record-attempt":
		return runRecordAttempt(args[1:])
	case "status":
		return runStatus(args[1:])
	case "stop":
		return runStop(args[1:])
	default:
		loopUsage()
		return fmt.Errorf("unknown loop subcommand %q", args[0])
	}
}

func loopUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness loop start --repo PATH --name NAME --goal TEXT [--max-attempts N] [--json] -- [VERIFY_ARGV...]
  agent-harness loop record-attempt --id ID --verdict pass|fail --evidence TEXT [--evidence TEXT...] [--json]
  agent-harness loop status (--id ID | --repo PATH --name NAME) [--json]
  agent-harness loop stop --id ID (--success | --reason TEXT) [--json]
`)
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("loop start", flag.ContinueOnError)
	repo := fs.String("repo", "", "repository path")
	name := fs.String("name", "", "loop name")
	goal := fs.String("goal", "", "loop goal")
	maxAttempts := fs.Int("max-attempts", 0, "maximum attempts before exhaustion")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StartLoopRun(core.LoopRunStartRequest{
		Repo:        *repo,
		Name:        *name,
		Goal:        *goal,
		MaxAttempts: *maxAttempts,
		VerifyArgv:  fs.Args(),
	})
	return printLoopResult(result, err, *jsonOut)
}

func runRecordAttempt(args []string) error {
	fs := flag.NewFlagSet("loop record-attempt", flag.ContinueOnError)
	id := fs.String("id", "", "loop id")
	verdict := fs.String("verdict", "", "attempt verdict: pass or fail")
	var evidence repeatedFlag
	fs.Var(&evidence, "evidence", "observable evidence")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.RecordLoopAttempt(*id, core.LoopRunRecordAttemptRequest{
		Verdict:  *verdict,
		Evidence: []string(evidence),
	})
	return printLoopResult(result, err, *jsonOut)
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("loop status", flag.ContinueOnError)
	id := fs.String("id", "", "loop id")
	repo := fs.String("repo", "", "repository path")
	name := fs.String("name", "", "loop name")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	loopID := *id
	var err error
	if loopID == "" {
		loopID, err = core.ResolveLoopRunID(*repo, *name)
		if err != nil {
			return printLoopResult(core.LoopRunStatusResult{OK: false}, err, *jsonOut)
		}
	}
	result, err := core.LoopRunStatus(loopID)
	return printLoopResult(result, err, *jsonOut)
}

func runStop(args []string) error {
	fs := flag.NewFlagSet("loop stop", flag.ContinueOnError)
	id := fs.String("id", "", "loop id")
	success := fs.Bool("success", false, "mark loop succeeded")
	reason := fs.String("reason", "", "stop reason for non-successful stop")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *success && *reason != "" {
		return printLoopResult(core.LoopRun{OK: false}, fmt.Errorf("success_and_reason_conflict"), *jsonOut)
	}
	result, err := core.StopLoopRun(*id, *success, *reason)
	return printLoopResult(result, err, *jsonOut)
}

func printLoopResult(result any, err error, jsonOut bool) error {
	if jsonOut {
		if err != nil {
			_ = printJSON(map[string]any{"ok": false, "error": err.Error()})
			return err
		}
		return printJSON(result)
	}
	if err != nil {
		return err
	}
	_ = printJSON(result)
	return nil
}

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
