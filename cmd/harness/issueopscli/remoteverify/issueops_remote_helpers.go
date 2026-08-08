package remoteverify

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

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
func runRemoteVerifyCommand(build func() *exec.Cmd) ([]byte, error) {
	attempts := remoteVerifyAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastOut []byte
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		out, err := build().Output()
		if err == nil {
			return out, nil
		}
		lastOut, lastErr = out, err
		if !isRetryableCommandError(err) || attempt == attempts-1 {
			break
		}
		if remoteVerifyBackoff > 0 {
			time.Sleep(remoteVerifyBackoff)
		}
	}
	return lastOut, lastErr
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
	if exitErr, ok := err.(*exec.ExitError); ok {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return fmt.Errorf("%s", stderr)
		}
	}
	return err
}
