package providerutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunBoundedReadbackRejectsOversizedOutputAndRedactsFailure(t *testing.T) {
	bin := t.TempDir()
	script := filepath.Join(bin, "readback")
	body := "#!/bin/sh\nif [ \"$1\" = large ]; then i=0; while [ $i -lt 270000 ]; do printf x; i=$((i+1)); done; exit 0; fi\nprintf 'api_key=abcdefghijklmnopqrstuvwxyz123456\\n' >&2\nexit 2\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := RunBoundedReadback(t.TempDir(), script, "large"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized readback error = %v", err)
	}
	if _, err := RunBoundedReadback(t.TempDir(), script, "secret"); err == nil || strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz123456") || len(err.Error()) > providerDiagnosticLimit+128 {
		t.Fatalf("secret readback diagnostic was not bounded/redacted: %v", err)
	}
}

func TestRunBoundedCommandReportsPostStartTimeoutWithoutRetryAuthority(t *testing.T) {
	script := filepath.Join(t.TempDir(), "slow")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, invoked, err := runBoundedCommand(t.TempDir(), script, nil, 20*time.Millisecond, 1024)
	if err == nil || !invoked || time.Since(started) > time.Second {
		t.Fatalf("timeout classification invoked=%v elapsed=%s err=%v", invoked, time.Since(started), err)
	}
}
