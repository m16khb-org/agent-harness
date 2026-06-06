package externalllm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunExternalLLMPrintTimeoutKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script timeout fixture is POSIX-only")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-agy.sh")
	script := `#!/bin/sh
(sleep 2; echo late-child-output) &
wait
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agy: %v", err)
	}

	started := time.Now()
	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Command: fake,
		Prompt:  "return json",
		Timeout: 50 * time.Millisecond,
	})
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got err=%v result=%+v", err, result)
	}
	if elapsed > 750*time.Millisecond {
		t.Fatalf("timeout should not wait for child process pipe close; elapsed=%s", elapsed)
	}
}
