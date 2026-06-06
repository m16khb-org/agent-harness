package policycli

import (
	"fmt"
	"os"

	"agent-harness/internal/core"
)

func runPolicy(args []string) error {
	if len(args) == 0 {
		policyUsage()
		return fmt.Errorf("missing policy subcommand")
	}
	switch args[0] {
	case "check":
		return runPolicyCheck(args[1:])
	case "fake-run":
		return runPolicyFakeRun(args[1:])
	case "run":
		return runPolicyRun(args[1:])
	case "audit":
		return runPolicyAudit(args[1:])
	default:
		policyUsage()
		return fmt.Errorf("unknown policy subcommand %q", args[0])
	}
}

func policyUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness policy check [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
  agent-harness policy fake-run [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
  agent-harness policy run --read-only [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--json] -- ARGV...
  agent-harness policy audit [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
`)
}

func runPolicyCheck(args []string) error {
	req, jsonOut, err := parseCommandPolicyFlags("policy check", args)
	if err != nil {
		return err
	}
	result := core.EvaluateCommandPolicy(req)
	if jsonOut {
		return printJSON(result)
	}
	printPolicyEvaluation(result)
	return nil
}

func runPolicyFakeRun(args []string) error {
	req, jsonOut, err := parseCommandPolicyFlags("policy fake-run", args)
	if err != nil {
		return err
	}
	result := core.FakeRunCommand(req)
	if jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		printPolicyEvaluation(result.Policy)
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}
	}
	if !result.Policy.Allowed {
		return core.PolicyDeniedError{Reasons: result.Policy.DenyReasons}
	}
	return nil
}

func runPolicyRun(args []string) error {
	req, jsonOut, readOnly, err := parseCommandPolicyRunFlags(args)
	if err != nil {
		return err
	}
	if !readOnly {
		return fmt.Errorf("policy run currently requires --read-only")
	}
	result := core.RunReadOnlyCommand(req)
	if jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		printPolicyEvaluation(result.Policy)
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}
	}
	if !result.Policy.Allowed {
		return core.PolicyDeniedError{Reasons: result.Policy.DenyReasons}
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("command exited %d", result.ExitCode)
	}
	return nil
}

func runPolicyAudit(args []string) error {
	req, jsonOut, err := parseCommandPolicyFlags("policy audit", args)
	if err != nil {
		return err
	}
	result, err := core.AuditCommandPolicy(req)
	if jsonOut {
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
	} else {
		printPolicyEvaluation(result.Policy)
		fmt.Printf("audit log: %s\n", result.LogPath)
	}
	return err
}

func printPolicyEvaluation(result core.CommandPolicyEvaluation) {
	if result.Allowed {
		fmt.Printf("policy allowed: %s\n", result.AuditLogID)
		return
	}
	fmt.Printf("policy denied: %s\n", result.AuditLogID)
	for _, reason := range result.DenyReasons {
		fmt.Printf("- %s\n", reason)
	}
}
