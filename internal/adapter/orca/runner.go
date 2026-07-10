package orca

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"agent-harness/internal/core/policy"
	"agent-harness/internal/port"
)

type CommandOutput struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Invoked  bool
}

type Runner interface {
	LookPath(string) (string, error)
	Run(context.Context, string, time.Duration, []string) (CommandOutput, error)
}

type ExecRunner struct{}

func (ExecRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (ExecRunner) Run(ctx context.Context, cwd string, timeout time.Duration, argv []string) (CommandOutput, error) {
	if len(argv) == 0 {
		return CommandOutput{}, &port.OrcaError{Code: "invalid_argv", Detail: "empty command"}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	cmd.Dir = strings.TrimSpace(cwd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return CommandOutput{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, &port.OrcaError{Code: "command_start_failed", Detail: boundedDiagnostic(err.Error()), Invoked: false}
	}
	output := CommandOutput{Invoked: true}
	err := cmd.Wait()
	output.Stdout = append([]byte(nil), stdout.Bytes()...)
	output.Stderr = append([]byte(nil), stderr.Bytes()...)
	if cmd.ProcessState != nil {
		output.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err == nil {
		return output, nil
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return output, &port.OrcaError{Code: "command_timeout", Detail: boundedDiagnostic(string(output.Stderr)), Invoked: true, Timeout: true}
	}
	return output, &port.OrcaError{Code: "command_failed", Detail: boundedDiagnostic(string(output.Stderr)), Invoked: true}
}

func boundedDiagnostic(value string) string {
	value = strings.TrimSpace(policy.RedactFreeform(value))
	const limit = 1024
	if len(value) > limit {
		return value[:limit] + "..."
	}
	return value
}
