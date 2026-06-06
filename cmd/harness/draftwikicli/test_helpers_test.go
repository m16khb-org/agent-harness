package draftwikicli

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
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = r.Close()
	})
	err = fn()
	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatal(copyErr)
	}
	if err != nil {
		t.Fatalf("function returned error: %v\nstdout:\n%s", err, buf.String())
	}
	return buf.String()
}

func captureProjectCLIStderr(fn func() error) (string, error) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stderr = w
	err = fn()
	closeErr := w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	_ = r.Close()
	if err != nil {
		return buf.String(), err
	}
	if closeErr != nil {
		return buf.String(), closeErr
	}
	return buf.String(), copyErr
}
