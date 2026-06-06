package issueopscli

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}

func captureProjectCLIStderr(fn func() error) (string, error) {
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close()
	os.Stderr = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		return "", closeErr
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		return "", err
	}
	return out.String(), callErr
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
