package providerutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"issueops/internal/domain/policy"
)

const (
	providerReadbackTimeout = 15 * time.Second
	providerMutationTimeout = 60 * time.Second
	providerReadbackLimit   = 256 * 1024
	providerDiagnosticLimit = 4096
)

func RunBoundedReadback(repo, name string, args ...string) ([]byte, error) {
	return RunBoundedReadbackContext(context.Background(), repo, name, args...)
}

func RunBoundedReadbackContext(ctx context.Context, repo, name string, args ...string) ([]byte, error) {
	stdout, _, err := runBoundedCommandContext(ctx, repo, name, args, providerReadbackTimeout, providerReadbackLimit)
	return stdout, err
}

func RunBoundedMutation(repo, name string, args ...string) (stdout []byte, invoked bool, err error) {
	return RunBoundedMutationContext(context.Background(), repo, name, args...)
}

func RunBoundedMutationContext(ctx context.Context, repo, name string, args ...string) (stdout []byte, invoked bool, err error) {
	return runBoundedCommandContext(ctx, repo, name, args, providerMutationTimeout, providerReadbackLimit)
}

func RunBoundedMutationWithOutputLimitContext(ctx context.Context, repo, name string, outputLimit int, args ...string) (stdout []byte, invoked bool, err error) {
	return runBoundedCommandContext(ctx, repo, name, args, providerMutationTimeout, outputLimit)
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
	return runBoundedCommandContext(context.Background(), repo, name, args, timeout, outputLimit)
}

func runBoundedCommandContext(parent context.Context, repo, name string, args []string, timeout time.Duration, outputLimit int) ([]byte, bool, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
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
	if ctxErr := ctx.Err(); ctxErr != nil {
		return output, true, fmt.Errorf("command failed after start: %w", ctxErr)
	}
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
	return policy.BoundedDiagnostic(value, limit)
}
