package vcsissue

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"agent-harness/cmd/harness/hookcli"
	"agent-harness/internal/testsupport"
)

func runPreToolUseCapture(t *testing.T, stdinJSON string, args ...string) map[string]any {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() { _, _ = io.WriteString(w, stdinJSON); _ = w.Close() }()
	defer func() { os.Stdin = oldStdin }()
	out := captureStdout(t, func() {
		if err := hookcli.RunHookPreToolUse(args); err != nil {
			t.Fatalf("hook: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook output is not JSON: %q: %v", out, err)
	}
	return obj
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return testsupport.CaptureStdout(t, func() error {
		fn()
		return nil
	})
}
