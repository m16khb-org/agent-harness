package remoteverify

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// withInstantBackoff drops the inter-attempt sleep so retry behaviour is tested
// without slowing the suite, restoring the production value afterwards.
func withInstantBackoff(t *testing.T) {
	t.Helper()
	previous := remoteVerifyBackoff
	remoteVerifyBackoff = 0
	t.Cleanup(func() { remoteVerifyBackoff = previous })
}

func TestRunRemoteVerifyCommandRetriesTransientFailureThenSucceeds(t *testing.T) {
	withInstantBackoff(t)
	bin := t.TempDir()
	countPath := filepath.Join(t.TempDir(), "count")
	// Fail transiently (HTTP 503, no auth/not-found signal) for the first
	// remoteVerifyAttempts-1 calls, then succeed on the final attempt.
	writeFakeCommand(t, filepath.Join(bin, "gh"), `#!/bin/sh
n=$(cat "$HARNESS_FAKE_COUNT" 2>/dev/null || echo 0)
n=$((n + 1))
printf '%s' "$n" > "$HARNESS_FAKE_COUNT"
if [ "$n" -lt `+strconv.Itoa(remoteVerifyAttempts)+` ]; then
  echo "HTTP 503: Service Unavailable" >&2
  exit 1
fi
printf '%s\n' '{"url":"https://github.com/example/repo/issues/7","state":"OPEN","title":"x"}'
exit 0
`)
	t.Setenv("HARNESS_FAKE_COUNT", countPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := VerifyGitHubIssueLive("https://github.com/example/repo/issues/7"); err != nil {
		t.Fatalf("expected transient failures to be retried until success, got %v", err)
	}
	if got, want := readCount(t, countPath), remoteVerifyAttempts; got != want {
		t.Fatalf("gh invocation count = %d, want %d (one per attempt up to success)", got, want)
	}
}

func TestRunRemoteVerifyCommandFailsFastOnAuthError(t *testing.T) {
	withInstantBackoff(t)
	bin := t.TempDir()
	countPath := filepath.Join(t.TempDir(), "count")
	// An auth-classified failure must NOT be retried so the documented MCP
	// fallback can engage immediately.
	writeFakeCommand(t, filepath.Join(bin, "gh"), `#!/bin/sh
n=$(cat "$HARNESS_FAKE_COUNT" 2>/dev/null || echo 0)
n=$((n + 1))
printf '%s' "$n" > "$HARNESS_FAKE_COUNT"
echo "HTTP 401: Bad credentials" >&2
exit 1
`)
	t.Setenv("HARNESS_FAKE_COUNT", countPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := VerifyGitHubIssueLive("https://github.com/example/repo/issues/7")
	if err == nil || !strings.Contains(err.Error(), "verify GitHub child issue through gh failed") {
		t.Fatalf("expected auth failure to surface, got %v", err)
	}
	if got := readCount(t, countPath); got != 1 {
		t.Fatalf("gh invocation count = %d, want 1 (auth error must fail fast without retry)", got)
	}
}

func readCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read invocation count: %v", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("invocation count file not numeric: %q", string(raw))
	}
	return n
}
