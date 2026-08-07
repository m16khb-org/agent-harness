package policycli

import (
	policy "agent-harness/internal/adapter/policy"
	"flag"
	"time"
)

func parseCommandPolicyFlags(name string, args []string) (policy.CommandPolicyRequest, bool, error) {
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
		return policy.CommandPolicyRequest{}, false, err
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

func parseCommandPolicyRunFlags(args []string) (policy.CommandPolicyRequest, bool, bool, error) {
	fs := flag.NewFlagSet("policy run", flag.ContinueOnError)
	workspaceRoot := fs.String("workspace-root", "", "workspace root boundary")
	cwd := fs.String("cwd", "", "command working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum runtime")
	envAllowlist := fs.String("env", "", "comma-separated environment variable allowlist")
	readOnly := fs.Bool("read-only", false, "execute only if policy allows a read-only command")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return policy.CommandPolicyRequest{}, false, false, err
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
	return req, *jsonOut, *readOnly, nil
}
