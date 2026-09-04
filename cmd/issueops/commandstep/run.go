package commandstep

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

func Run(dir, label string, timeout time.Duration, stdin string, outputBudget int, name string, args ...string) StepResult {
	return RunEnv(dir, label, timeout, stdin, nil, outputBudget, name, args...)
}

func RunEnv(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
	return RunEnvWithBudget(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func RunEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = MergeEnvOverrides(os.Environ(), env)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	stdoutText, stdoutTruncated, stdoutBytes := BudgetCommandOutput(stdout.String(), outputBudget)
	stderrText, stderrTruncated, stderrBytes := BudgetCommandOutput(stderr.String(), outputBudget)
	step := StepResult{
		Label:           label,
		Command:         strings.Join(append([]string{name}, args...), " "),
		OK:              err == nil,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		Stderr:          stderrText,
		StdoutBytes:     stdoutBytes,
		StderrBytes:     stderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}
	if ctx.Err() == context.DeadlineExceeded {
		step.OK = false
		step.Error = "timeout after " + timeout.String()
	} else if err != nil {
		step.Error = err.Error()
	}
	return step
}
