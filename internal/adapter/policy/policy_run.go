package policy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func FakeRunCommand(req CommandPolicyRequest) CommandFakeRunResult {
	started := time.Now()
	policy := EvaluateCommandPolicy(req)
	finished := time.Now()
	result := CommandFakeRunResult{
		OK:         policy.Allowed,
		Executed:   false,
		ExitCode:   0,
		StartedAt:  started.UTC().Format(time.RFC3339Nano),
		FinishedAt: finished.UTC().Format(time.RFC3339Nano),
		DurationMS: finished.Sub(started).Milliseconds(),
		Policy:     policy,
	}
	if !policy.Allowed {
		result.ExitCode = 3
		result.Stderr = "fake-run denied by policy: " + strings.Join(policy.DenyReasons, "; ") + "\n"
		return result
	}
	result.Stdout = fmt.Sprintf("fake-run accepted by policy; command was not executed\nargv: %s\naudit_log_id: %s\n", strings.Join(policy.Argv, " "), policy.AuditLogID)
	return result
}

func RunReadOnlyCommand(req CommandPolicyRequest) CommandRunResult {
	req.WriteAllowed = false
	req.NetworkAllowed = false
	req.ShellAllowed = false
	started := time.Now()
	policy := EvaluateCommandPolicy(req)
	result := CommandRunResult{
		OK:        policy.Allowed,
		Executed:  false,
		ExitCode:  0,
		StartedAt: started.UTC().Format(time.RFC3339Nano),
		ReadOnly:  true,
		Policy:    policy,
	}
	if !policy.Allowed {
		finished := time.Now()
		result.ExitCode = 3
		result.FinishedAt = finished.UTC().Format(time.RFC3339Nano)
		result.DurationMS = finished.Sub(started).Milliseconds()
		result.Stderr = "run denied by policy: " + strings.Join(policy.DenyReasons, "; ") + "\n"
		return result
	}
	timeout := 30 * time.Second
	if parsed, err := time.ParseDuration(req.Timeout); err == nil && parsed > 0 {
		timeout = parsed
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, req.Argv[0], req.Argv[1:]...)
	cmd.Dir = req.CWD
	cmd.Env = commandEnv(req.EnvAllowlist)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	finished := time.Now()
	result.Executed = true
	result.FinishedAt = finished.UTC().Format(time.RFC3339Nano)
	result.DurationMS = finished.Sub(started).Milliseconds()
	result.TimedOut = ctx.Err() == context.DeadlineExceeded
	result.Stdout = budgetOutput(redactFreeform(stdout.String()))
	result.Stderr = budgetOutput(redactFreeform(stderr.String()))
	if err != nil {
		result.OK = false
		result.ExitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		if result.TimedOut {
			result.ExitCode = 124
			if result.Stderr != "" && !strings.HasSuffix(result.Stderr, "\n") {
				result.Stderr += "\n"
			}
			result.Stderr += "command timed out\n"
		}
		return result
	}
	result.OK = true
	return result
}

func commandEnv(allowlist []string) []string {
	allowed := map[string]bool{}
	for _, name := range cleanEnvAllowlist(allowlist) {
		allowed[name] = true
	}
	env := []string{}
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			env = append(env, entry)
		}
	}
	return env
}

func budgetOutput(text string) string {
	const limit = 32 * 1024
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "\n<truncated>\n"
}
