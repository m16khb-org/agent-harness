package basiccli

import (
	guard "agent-harness/internal/adapter/guard"
	guardcontract "agent-harness/internal/contract/guard"
	"flag"
	"fmt"
	"os"
)

func runGuard(args []string) error {
	if len(args) == 0 {
		guardUsage()
		return fmt.Errorf("missing guard subcommand")
	}
	switch args[0] {
	case "check":
		return runGuardCheck(args[1:])
	default:
		guardUsage()
		return fmt.Errorf("unknown guard subcommand %q", args[0])
	}
}

func guardUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness guard check [--repo PATH] [--staged] [--all] [--json] [--] [FILES...]
`)
}

func runGuardCheck(args []string) error {
	fs := flag.NewFlagSet("guard check", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	staged := fs.Bool("staged", true, "check staged files")
	all := fs.Bool("all", false, "check all relevant files")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()
	if len(files) > 0 {
		*staged = false
	}
	if *all {
		*staged = false
	}
	result := GuardCheck(guardcontract.GuardCheckRequest{RepoRoot: *repo, Staged: *staged, All: *all, Files: files})
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		fmt.Printf("guard %s: %d file(s), block=%d warn=%d review=%d\n", result.Mode, len(result.CheckedFiles), result.Summary.Block, result.Summary.Warn, result.Summary.Review)
		for _, finding := range result.Findings {
			location := finding.File
			if finding.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, finding.Line)
			}
			if location == "" {
				location = "-"
			}
			fmt.Printf("%s %s %s: %s\n", finding.Severity, finding.Rule, location, finding.Message)
		}
	}
	if !result.OK {
		blockers := []guardcontract.GuardFinding{}
		for _, finding := range result.Findings {
			if finding.Severity == "block" {
				blockers = append(blockers, finding)
			}
		}
		return guard.GuardBlockedError{Findings: blockers}
	}
	return nil
}
