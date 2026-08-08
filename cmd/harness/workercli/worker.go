package workercli

import (
	policy "agent-harness/internal/contract/policy"
	"flag"
	"fmt"
	"os"
	"time"
)

func runWorker(args []string) error {
	if len(args) == 0 {
		workerUsage()
		return fmt.Errorf("missing worker subcommand")
	}
	switch args[0] {
	case "enqueue":
		return runWorkerEnqueue(args[1:])
	case "run":
		return runWorkerRun(args[1:])
	case "status":
		return runWorkerStatus(args[1:])
	case "list":
		return runWorkerList(args[1:])
	case "cleanup-stuck":
		return runWorkerCleanupStuck(args[1:])
	case "cancel":
		return runWorkerCancel(args[1:])
	default:
		workerUsage()
		return fmt.Errorf("unknown worker subcommand %q", args[0])
	}
}

func workerUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness worker enqueue --kind KIND [--payload TEXT] [--json]
  agent-harness worker run --read-only --kind KIND [--payload TEXT] [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--json] -- ARGV...
  agent-harness worker status --id ID [--json]
  agent-harness worker list [--json]
  agent-harness worker cleanup-stuck [--json]
  agent-harness worker cancel --id ID [--json]
`)
}

func runWorkerRun(args []string) error {
	fs := flag.NewFlagSet("worker run", flag.ContinueOnError)
	kind := fs.String("kind", "read-only-command", "job kind")
	payload := fs.String("payload", "", "redacted job payload")
	readOnly := fs.Bool("read-only", false, "execute an argv-only read-only command")
	workspaceRoot := fs.String("workspace-root", "", "workspace root boundary")
	cwd := fs.String("cwd", "", "command working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum runtime")
	envAllowlist := fs.String("env", "", "comma-separated environment variable allowlist")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*readOnly {
		return fmt.Errorf("worker run currently requires --read-only")
	}
	root := *workspaceRoot
	if root == "" {
		root = deps.ResolveTarget("")
	}
	workDir := *cwd
	if workDir == "" {
		workDir = root
	}
	req := policy.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           workDir,
		Argv:          fs.Args(),
		Timeout:       timeout.String(),
		EnvAllowlist:  splitCSV(*envAllowlist),
	}
	job, err := RunReadOnlyWorkerJob(*kind, *payload, req)
	if *jsonOut {
		_ = printJSON(job)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("%s %s exit=%d\n", job.ID, job.Status, job.Result.ExitCode)
		if job.Result.Stdout != "" {
			fmt.Print(job.Result.Stdout)
		}
		if job.Result.Stderr != "" {
			fmt.Fprint(os.Stderr, job.Result.Stderr)
		}
	}
	if err != nil {
		return err
	}
	if job.Result != nil && !job.Result.Policy.Allowed {
		return policy.PolicyDeniedError{Reasons: job.Result.Policy.DenyReasons}
	}
	if job.Result != nil && job.Result.ExitCode != 0 {
		return fmt.Errorf("worker read-only command exited %d", job.Result.ExitCode)
	}
	return nil
}
