package orca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestExecRunnerRetriesOwnerlessInheritedRelayWithCurrentWrapper(t *testing.T) {
	t.Setenv("ORCA_RELAY_DIR", "/stale/relay")
	t.Setenv("ORCA_RELAY_SOCKET_PATH", "/stale/relay.sock")
	t.Setenv("ORCA_RELAY_CREDENTIAL_FILE", "/stale/relay.credential")
	t.Setenv("ORCA_RELAY_NODE_PATH", "/current/node")
	t.Setenv("ORCA_ENVIRONMENT", "remote-fixture")
	t.Setenv("ORCA_PAIRING_CODE", "pairing-fixture")
	attemptLog := filepath.Join(t.TempDir(), "attempts")
	t.Setenv("AGENT_HARNESS_ORCA_ATTEMPT_LOG", attemptLog)
	t.Setenv("AGENT_HARNESS_ORCA_STREAM_HELPER", "relay-retry")

	output, err := (ExecRunner{}).Run(context.Background(), "", 5*time.Second, []string{os.Args[0], "-test.run=^TestExecRunnerStreamHelper$"})
	if err != nil {
		t.Fatal(err)
	}
	const want = "relay_dir=false relay_socket=false relay_credential=false relay_node=/current/node environment=remote-fixture pairing=pairing-fixture"
	if got := strings.TrimSpace(string(output.Stdout)); got != want {
		t.Fatalf("Orca child environment = %q, want %q", got, want)
	}
	attempts, err := os.ReadFile(attemptLog)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(attempts), "inherited\ncurrent\n"; got != want {
		t.Fatalf("Orca relay attempts = %q, want %q", got, want)
	}
	if output.ExitCode != 0 || !output.Invoked {
		t.Fatalf("Orca retry output = %#v, want successful invoked command", output)
	}
}

func TestExecRunnerKeepsResponsiveInheritedRelay(t *testing.T) {
	t.Setenv("ORCA_RELAY_DIR", "/responsive/relay")
	t.Setenv("ORCA_RELAY_SOCKET_PATH", "/responsive/relay.sock")
	t.Setenv("ORCA_RELAY_CREDENTIAL_FILE", "/responsive/relay.credential")
	t.Setenv("ORCA_RELAY_NODE_PATH", "")
	t.Setenv("ORCA_ENVIRONMENT", "")
	t.Setenv("ORCA_PAIRING_CODE", "")
	attemptLog := filepath.Join(t.TempDir(), "attempts")
	t.Setenv("AGENT_HARNESS_ORCA_ATTEMPT_LOG", attemptLog)
	t.Setenv("AGENT_HARNESS_ORCA_STREAM_HELPER", "relay-live")

	output, err := (ExecRunner{}).Run(context.Background(), "", 5*time.Second, []string{os.Args[0], "-test.run=^TestExecRunnerStreamHelper$"})
	if err != nil {
		t.Fatal(err)
	}
	const want = "relay_dir=true relay_socket=true relay_credential=true relay_node= environment= pairing="
	if got := strings.TrimSpace(string(output.Stdout)); got != want {
		t.Fatalf("Orca child environment = %q, want %q", got, want)
	}
	attempts, err := os.ReadFile(attemptLog)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(attempts), "inherited\n"; got != want {
		t.Fatalf("Orca relay attempts = %q, want %q", got, want)
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
	if stream == "relay-retry" || stream == "relay-live" {
		relay := "current"
		if os.Getenv("ORCA_RELAY_DIR") != "" {
			relay = "inherited"
		}
		attemptLog := os.Getenv("AGENT_HARNESS_ORCA_ATTEMPT_LOG")
		file, err := os.OpenFile(attemptLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(2)
		}
		if _, err := file.WriteString(relay + "\n"); err != nil {
			_ = file.Close()
			os.Exit(2)
		}
		if err := file.Close(); err != nil {
			os.Exit(2)
		}
		if stream == "relay-retry" && relay == "inherited" {
			_, _ = os.Stderr.WriteString("No owning Orca client is connected to the relay")
			os.Exit(1)
		}
		_, _ = os.Stdout.WriteString("relay_dir=" + presence(os.Getenv("ORCA_RELAY_DIR")) +
			" relay_socket=" + presence(os.Getenv("ORCA_RELAY_SOCKET_PATH")) +
			" relay_credential=" + presence(os.Getenv("ORCA_RELAY_CREDENTIAL_FILE")) +
			" relay_node=" + os.Getenv("ORCA_RELAY_NODE_PATH") +
			" environment=" + os.Getenv("ORCA_ENVIRONMENT") +
			" pairing=" + os.Getenv("ORCA_PAIRING_CODE"))
		os.Exit(0)
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

func presence(value string) string {
	if value == "" {
		return "false"
	}
	return "true"
}
