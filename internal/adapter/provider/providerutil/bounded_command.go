package providerutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"agent-harness/internal/domain/policy"
)

const (
	providerReadbackTimeout = 15 * time.Second
	providerMutationTimeout = 60 * time.Second
	providerReadbackLimit   = 256 * 1024
	providerDiagnosticLimit = 4096
)

func RunBoundedReadback(repo, name string, args ...string) ([]byte, error) {
	stdout, _, err := runBoundedCommand(repo, name, args, providerReadbackTimeout, providerReadbackLimit)
	return stdout, err
}

func RunBoundedMutation(repo, name string, args ...string) (stdout []byte, invoked bool, err error) {
	return runBoundedCommand(repo, name, args, providerMutationTimeout, providerReadbackLimit)
}

func DryRunPreview(name string, args ...string) string {
	argv := append([]string{name}, args...)
	value := strings.Join(policy.RedactArgv(argv), " ")
	if len(value) > providerDiagnosticLimit {
		value = value[:providerDiagnosticLimit] + "...[truncated]"
	}
	return "[dry-run] would execute: " + value
}

func runBoundedCommand(repo, name string, args []string, timeout time.Duration, outputLimit int) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = repo
	stdout := &boundedBuffer{limit: outputLimit}
	stderr := &boundedBuffer{limit: providerDiagnosticLimit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Start(); err != nil {
		return nil, false, fmt.Errorf("command start failed: %s", boundedDiagnostic(err.Error()))
	}
	err := cmd.Wait()
	output := append([]byte(nil), stdout.data...)
	if stdout.truncated {
		return output, true, fmt.Errorf("command output exceeds %d bytes", outputLimit)
	}
	if err != nil {
		return output, true, fmt.Errorf("command failed after start: %s", boundedDiagnostic(stderr.String()+" "+err.Error()))
	}
	return output, true, nil
}

type boundedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	if n > remaining {
		b.truncated = true
	}
	return n, nil
}

func (b *boundedBuffer) String() string {
	return string(b.data)
}

func boundedDiagnostic(value string) string {
	return BoundedDiagnostic(value, providerDiagnosticLimit)
}

func BoundedDiagnostic(value string, limit int) string {
	if limit <= 0 || limit > providerDiagnosticLimit {
		limit = providerDiagnosticLimit
	}
	value = strings.TrimSpace(policy.RedactDiagnostic(value))
	if len(value) > limit {
		value = value[:limit] + "...[truncated]"
	}
	return value
}
