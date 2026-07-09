package testsupport

import (
	"io"
	"os"
	"testing"
)

// CaptureStdout captures os.Stdout for test helpers without depending on pipe buffer capacity.
func CaptureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	out, err := CaptureStdoutAndError(t, fn)
	if err != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", err, out)
	}
	return out
}

// CaptureStdoutAndError captures os.Stdout and returns the function error to the caller.
func CaptureStdoutAndError(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	type readResult struct {
		out []byte
		err error
	}
	readDone := make(chan readResult, 1)
	go func() {
		out, err := io.ReadAll(r)
		readDone <- readResult{out: out, err: err}
	}()
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	read := <-readDone
	if read.err != nil {
		t.Fatal(read.err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return string(read.out), runErr
}
