package orca

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"agent-harness/internal/domain/policy"
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

type boundedStreamBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedStreamBuffer) Write(value []byte) (int, error) {
	written := len(value)
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.exceeded = b.exceeded || written > 0
		return written, nil
	}
	if len(value) > remaining {
		value = value[:remaining]
		b.exceeded = true
	}
	_, err := b.buffer.Write(value)
	return written, err
}

func (b *boundedStreamBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func (ExecRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (ExecRunner) Run(ctx context.Context, cwd string, timeout time.Duration, argv []string) (CommandOutput, error) {
	if len(argv) == 0 {
		return CommandOutput{}, &port.OrcaError{Code: "invalid_argv", Detail: "empty command"}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	environ := os.Environ()
	output, err := runOrcaCommand(runCtx, cwd, argv, nil)
	if !shouldRetryWithoutInheritedRelay(environ, output, err) {
		return output, err
	}
	return runOrcaCommand(runCtx, cwd, argv, orcaCommandEnvironment(environ))
}

func runOrcaCommand(ctx context.Context, cwd string, argv []string, environ []string) (CommandOutput, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = strings.TrimSpace(cwd)
	if environ != nil {
		cmd.Env = environ
	}
	stdout := boundedStreamBuffer{limit: MaxEnvelopeBytes}
	stderr := boundedStreamBuffer{limit: MaxEnvelopeBytes}
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
	if stdout.exceeded || stderr.exceeded {
		return output, &port.OrcaError{Code: "command_output_too_large", Detail: "Orca stdout or stderr exceeded the bounded stream limit", Invoked: true}
	}
	if err == nil {
		return output, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return output, &port.OrcaError{Code: "command_timeout", Detail: boundedDiagnostic(string(output.Stderr)), Invoked: true, Timeout: true}
	}
	return output, &port.OrcaError{Code: "command_failed", Detail: commandFailureDiagnostic(output), Invoked: true}
}

func shouldRetryWithoutInheritedRelay(environ []string, output CommandOutput, err error) bool {
	const ownerlessRelayDiagnostic = "No owning Orca client is connected to the relay"
	if !hasInheritedOrcaRelay(environ) {
		return false
	}
	orcaErr, ok := errors.AsType[*port.OrcaError](err)
	return ok &&
		orcaErr.Code == "command_failed" &&
		strings.Contains(string(output.Stderr), ownerlessRelayDiagnostic)
}

func hasInheritedOrcaRelay(environ []string) bool {
	present := make(map[string]bool, 3)
	for _, entry := range environ {
		name, value, found := strings.Cut(entry, "=")
		if found && value != "" {
			switch name {
			case "ORCA_RELAY_DIR", "ORCA_RELAY_SOCKET_PATH", "ORCA_RELAY_CREDENTIAL_FILE":
				present[name] = true
			}
		}
	}
	return present["ORCA_RELAY_DIR"] &&
		present["ORCA_RELAY_SOCKET_PATH"] &&
		present["ORCA_RELAY_CREDENTIAL_FILE"]
}

func orcaCommandEnvironment(environ []string) []string {
	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "ORCA_RELAY_DIR", "ORCA_RELAY_SOCKET_PATH", "ORCA_RELAY_CREDENTIAL_FILE":
			continue
		default:
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func commandFailureDiagnostic(output CommandOutput) string {
	stdout := boundedDiagnostic(string(output.Stdout))
	stderr := boundedDiagnostic(string(output.Stderr))
	switch {
	case stdout != "" && stderr != "":
		return "stdout: " + stdout + "; stderr: " + stderr
	case stdout != "":
		return "stdout: " + stdout
	default:
		return stderr
	}
}

func boundedDiagnostic(value string) string {
	value = strings.TrimSpace(policy.RedactFreeform(value))
	const limit = 1024
	if len(value) > limit {
		return value[:limit] + "..."
	}
	return value
}
