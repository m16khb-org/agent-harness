package remoteverify

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	policydomain "issueops/internal/domain/policy"
)

const maxRemoteVerifyDiagnosticBytes = 2048
const maxRemoteVerifyOutputBytes = 256 * 1024
const remoteVerifyCommandTimeout = 30 * time.Second

// remoteVerifyAttempts is the bounded number of times a load-bearing gh/glab
// verification exec is attempted before a transient failure is surfaced. It is a
// package var so tests can tune it; the load-bearing verify must not hang on a
// single network blip the way the optional LLM judge never did.
var remoteVerifyAttempts = 3

// remoteVerifyBackoff is the delay between transient retries. Kept tiny and
// deterministic so the retry never meaningfully slows verification or tests; a
// momentary 5xx/rate-limit resolves well within this window.
var remoteVerifyBackoff = 50 * time.Millisecond

// runRemoteVerifyCommand runs a gh/glab verification command with bounded retry
// and returns the raw command result so callers keep wrapping failures through
// commandOutputError. A transient failure (bare non-zero exit, 5xx, rate limit,
// network blip) is retried up to remoteVerifyAttempts times with a short
// backoff; an auth/permission/missing-credential, missing-binary, or
// definitively-not-found failure is returned immediately (fail-fast) so the
// documented fallback (MCP, or the work_items->issues fallback) can engage
// without burning retries.
func runRemoteVerifyCommand(ctx context.Context, build func(context.Context) *exec.Cmd) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, remoteVerifyCommandTimeout)
	defer cancel()
	attempts := remoteVerifyAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastOut []byte
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out, err := executeBoundedRemoteVerifyCommand(build(ctx))
		if err == nil {
			return out, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return out, ctxErr
		}
		lastOut, lastErr = out, err
		if !isRetryableCommandError(err) || attempt == attempts-1 {
			break
		}
		if remoteVerifyBackoff > 0 {
			timer := time.NewTimer(remoteVerifyBackoff)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return lastOut, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return lastOut, lastErr
}

func executeBoundedRemoteVerifyCommand(cmd *exec.Cmd) ([]byte, error) {
	stdout := &remoteVerifyBuffer{limit: maxRemoteVerifyOutputBytes}
	stderr := &remoteVerifyBuffer{limit: maxRemoteVerifyDiagnosticBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	err := cmd.Wait()
	if stdout.truncated {
		return append([]byte(nil), stdout.data...),
			fmt.Errorf("remote verification output exceeds %d bytes", maxRemoteVerifyOutputBytes)
	}
	if err != nil {
		diagnostic := policydomain.BoundedDiagnostic(stderr.String()+" "+err.Error(), maxRemoteVerifyDiagnosticBytes)
		return append([]byte(nil), stdout.data...), &remoteVerifyCommandError{
			cause:          err,
			diagnostic:     diagnostic,
			classification: strings.ToLower(stderr.String()),
		}
	}
	return append([]byte(nil), stdout.data...), nil
}

type remoteVerifyBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (buffer *remoteVerifyBuffer) Write(value []byte) (int, error) {
	size := len(value)
	remaining := buffer.limit - len(buffer.data)
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		buffer.data = append(buffer.data, value...)
	}
	if size > remaining {
		buffer.truncated = true
	}
	return size, nil
}

func (buffer *remoteVerifyBuffer) String() string {
	return string(buffer.data)
}

type remoteVerifyCommandError struct {
	cause          error
	diagnostic     string
	classification string
}

func (err *remoteVerifyCommandError) Error() string {
	return err.diagnostic
}

func (err *remoteVerifyCommandError) Unwrap() error {
	return err.cause
}

// nonRetryableCommandSignals are lowercased stderr fragments that mark a gh/glab
// failure as definitive: auth/permission/credential problems and missing
// resources where retrying the identical command cannot change the outcome.
var nonRetryableCommandSignals = []string{
	"http 401",
	"http 403",
	"http 404",
	"401 unauthorized",
	"403 forbidden",
	"unauthorized",
	"forbidden",
	"not found",
	"authentication",
	"not logged in",
	"auth login",
	"auth status",
	"credential",
	"permission denied",
	"not installed",
}

// isRetryableCommandError reports whether a failed gh/glab verification command
// should be retried. A missing or non-executable binary, and any failure whose
// stderr carries an auth/permission/missing-resource signal, is NOT retryable:
// the fix is the documented fallback, not another identical attempt. Everything
// else — bare non-zero exits and transient 5xx/rate-limit/network blips — is
// retried.
func isRetryableCommandError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := errors.AsType[*exec.Error](err); ok {
		// Binary missing or not executable on PATH: retrying won't help.
		return false
	}
	if bounded, ok := errors.AsType[*remoteVerifyCommandError](err); ok {
		for _, signal := range nonRetryableCommandSignals {
			if strings.Contains(bounded.classification, signal) {
				return false
			}
		}
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		stderr := strings.ToLower(string(exitErr.Stderr))
		for _, signal := range nonRetryableCommandSignals {
			if strings.Contains(stderr, signal) {
				return false
			}
		}
		return true
	}
	// Unknown error shape without exit metadata: treat as transient.
	return true
}

func requireRemoteValues(kind string, required []string, actual []string) error {
	actualSet := map[string]bool{}
	for _, value := range actual {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			actualSet[value] = true
		}
	}
	missing := []string{}
	for _, value := range required {
		cleaned := strings.TrimSpace(strings.ToLower(value))
		if cleaned != "" && !actualSet[cleaned] {
			missing = append(missing, value)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("remote artifact missing verified %s(s): %s", kind, strings.Join(missing, ", "))
	}
	return nil
}

func commandOutputError(err error) error {
	if bounded, ok := errors.AsType[*remoteVerifyCommandError](err); ok {
		return fmt.Errorf("%s", policydomain.BoundedDiagnostic(bounded.diagnostic, maxRemoteVerifyDiagnosticBytes))
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			diagnostic := policydomain.RedactDiagnostic(stderr)
			if len(diagnostic) > maxRemoteVerifyDiagnosticBytes {
				diagnostic = diagnostic[:maxRemoteVerifyDiagnosticBytes]
			}
			return fmt.Errorf("%s", diagnostic)
		}
	}
	return err
}
