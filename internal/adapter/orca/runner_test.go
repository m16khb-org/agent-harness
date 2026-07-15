package orca

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/port"
)

func TestExecRunnerBoundsStreamsWhileCommandRuns(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			t.Setenv("AGENT_HARNESS_ORCA_STREAM_HELPER", stream)
			output, err := (ExecRunner{}).Run(context.Background(), "", 5*time.Second, []string{os.Args[0], "-test.run=^TestExecRunnerStreamHelper$"})
			var orcaErr *port.OrcaError
			if !errors.As(err, &orcaErr) || orcaErr.Code != "command_output_too_large" || !orcaErr.Invoked {
				t.Fatalf("oversized %s error = %#v, want invoked command_output_too_large", stream, err)
			}
			if len(output.Stdout) > MaxEnvelopeBytes || len(output.Stderr) > MaxEnvelopeBytes {
				t.Fatalf("runner retained oversized streams: stdout=%d stderr=%d", len(output.Stdout), len(output.Stderr))
			}
		})
	}
}

func TestExecRunnerIncludesBoundedStdoutInCommandFailureDiagnostic(t *testing.T) {
	t.Setenv("AGENT_HARNESS_ORCA_STREAM_HELPER", "failure-json")
	_, err := (ExecRunner{}).Run(context.Background(), "", 5*time.Second, []string{os.Args[0], "-test.run=^TestExecRunnerStreamHelper$"})
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "command_failed" || !orcaErr.Invoked || !strings.Contains(orcaErr.Detail, "stdout: {\"error\":\"launch_failed\"}") || !strings.Contains(orcaErr.Detail, "stderr: relay handshake") {
		t.Fatalf("failure diagnostic = %#v", err)
	}
}

func TestExecRunnerStreamHelper(t *testing.T) {
	stream := os.Getenv("AGENT_HARNESS_ORCA_STREAM_HELPER")
	if stream == "" {
		return
	}
	if stream == "failure-json" {
		_, _ = os.Stdout.WriteString(`{"error":"launch_failed"}`)
		_, _ = os.Stderr.WriteString("relay handshake")
		os.Exit(1)
	}
	w := os.Stdout
	if stream == "stderr" {
		w = os.Stderr
	}
	chunk := make([]byte, 64*1024)
	for i := range chunk {
		chunk[i] = 'x'
	}
	remaining := MaxEnvelopeBytes + len(chunk)
	for remaining > 0 {
		write := len(chunk)
		if remaining < write {
			write = remaining
		}
		if _, err := w.Write(chunk[:write]); err != nil {
			os.Exit(2)
		}
		remaining -= write
	}
	os.Exit(0)
}
