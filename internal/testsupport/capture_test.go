package testsupport

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestCaptureStdoutLargeOutputDoesNotBlock(t *testing.T) {
	payload := strings.Repeat("x", 64*1024)

	out := CaptureStdout(t, func() error {
		fmt.Print(payload)
		return nil
	})

	if out != payload {
		t.Fatalf("captured stdout mismatch: got %d bytes want %d", len(out), len(payload))
	}
}

func TestCaptureStdoutAndErrorReturnsFunctionErrorWithOutput(t *testing.T) {
	wantErr := errors.New("boom")

	out, err := CaptureStdoutAndError(t, func() error {
		fmt.Print("partial output")
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected function error, got %v", err)
	}
	if out != "partial output" {
		t.Fatalf("captured stdout = %q, want partial output", out)
	}
}

func TestCaptureStdoutRestoresStdout(t *testing.T) {
	oldStdout := os.Stdout

	out := CaptureStdout(t, func() error {
		fmt.Print("captured")
		return nil
	})

	if out != "captured" {
		t.Fatalf("captured stdout = %q, want captured", out)
	}
	if os.Stdout != oldStdout {
		t.Fatal("CaptureStdout did not restore os.Stdout")
	}
}

func TestCaptureStderrAndErrorReturnsFunctionErrorWithOutput(t *testing.T) {
	wantErr := errors.New("boom")

	out, err := CaptureStderrAndError(t, func() error {
		fmt.Fprint(os.Stderr, "partial error")
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected function error, got %v", err)
	}
	if out != "partial error" {
		t.Fatalf("captured stderr = %q, want partial error", out)
	}
}

func TestCaptureStderrRestoresStderr(t *testing.T) {
	oldStderr := os.Stderr

	out := CaptureStderr(t, func() error {
		fmt.Fprint(os.Stderr, "captured")
		return nil
	})

	if out != "captured" {
		t.Fatalf("captured stderr = %q, want captured", out)
	}
	if os.Stderr != oldStderr {
		t.Fatal("CaptureStderr did not restore os.Stderr")
	}
}
