package main

import (
	"flag"
	"fmt"
	"os"
	"time"

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

func parseCommandPolicyFlags(name string, args []string) (core.CommandPolicyRequest, bool, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	workspaceRoot := fs.String("workspace-root", "", "workspace root boundary")
	cwd := fs.String("cwd", "", "command working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum runtime")
	envAllowlist := fs.String("env", "", "comma-separated environment variable allowlist")
	writeAllowed := fs.Bool("write", false, "allow workspace writes")
	networkAllowed := fs.Bool("network", false, "allow network access")
	shellAllowed := fs.Bool("shell", false, "allow shell interpreter argv[0] with --shell-reason")
	shellReason := fs.String("shell-reason", "", "reason for shell interpreter exception")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return core.CommandPolicyRequest{}, false, err
	}
	root := *workspaceRoot
	if root == "" {
		root = resolveTarget("")
	}
	workDir := *cwd
	if workDir == "" {
		workDir = root
	}
	req := core.CommandPolicyRequest{
		WorkspaceRoot:  root,
		CWD:            workDir,
		Argv:           fs.Args(),
		Timeout:        timeout.String(),
		EnvAllowlist:   splitCSV(*envAllowlist),
		NetworkAllowed: *networkAllowed,
		WriteAllowed:   *writeAllowed,
		ShellAllowed:   *shellAllowed,
		ShellReason:    *shellReason,
	}
	return req, *jsonOut, nil
}

func parseCommandPolicyRunFlags(args []string) (core.CommandPolicyRequest, bool, bool, error) {
	fs := flag.NewFlagSet("policy run", flag.ContinueOnError)
	workspaceRoot := fs.String("workspace-root", "", "workspace root boundary")
	cwd := fs.String("cwd", "", "command working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum runtime")
	envAllowlist := fs.String("env", "", "comma-separated environment variable allowlist")
	readOnly := fs.Bool("read-only", false, "execute only if policy allows a read-only command")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return core.CommandPolicyRequest{}, false, false, err
	}
	root := *workspaceRoot
	if root == "" {
		root = resolveTarget("")
	}
	workDir := *cwd
	if workDir == "" {
		workDir = root
	}
	req := core.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           workDir,
		Argv:          fs.Args(),
		Timeout:       timeout.String(),
		EnvAllowlist:  splitCSV(*envAllowlist),
	}
	return req, *jsonOut, *readOnly, nil
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
