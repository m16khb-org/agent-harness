package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunRootCommandUsageAndVersionSurface(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "no args prints usage to stderr",
			args:     []string{},
			wantCode: 2,
			wantErr:  "agent-harness 0.1.0",
		},
		{
			name:     "help prints usage to stderr",
			args:     []string{"help"},
			wantCode: 0,
			wantErr:  "agent-harness 0.1.0",
		},
		{
			name:     "version prints stdout",
			args:     []string{"version"},
			wantCode: 0,
			wantOut:  "agent-harness 0.1.0\n",
		},
		{
			name:     "unknown prints usage to stderr",
			args:     []string{"unknown-command"},
			wantCode: 2,
			wantErr:  "agent-harness 0.1.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errText, code := captureRootCommandOutput(t, tc.args)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tc.wantCode)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Fatalf("stdout = %q, want containing %q", out, tc.wantOut)
			}
			if !strings.Contains(errText, tc.wantErr) {
				t.Fatalf("stderr = %q, want containing %q", errText, tc.wantErr)
			}
		})
	}
}

func captureRootCommandOutput(t *testing.T, args []string) (string, string, int) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	os.Stdout = outWrite
	os.Stderr = errWrite
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	code := runRootCommand(args)
	if err := outWrite.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	if err := errWrite.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}

	outBytes, err := io.ReadAll(outRead)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	errBytes, err := io.ReadAll(errRead)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}

	return string(outBytes), string(errBytes), code
}
