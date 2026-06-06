package daemoncli

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRunDaemonRejectsMissingAndUnknownSubcommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing", args: nil, wantErr: "missing daemon subcommand"},
		{name: "unknown", args: []string{"missing-command"}, wantErr: `unknown daemon subcommand "missing-command"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr, err := captureProjectCLIStderr(func() error {
				return runDaemon(tt.args)
			})

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
			if !strings.Contains(stderr, "agent-harness daemon start") || !strings.Contains(stderr, "agent-harness daemon stop") {
				t.Fatalf("expected daemon usage on stderr, got:\n%s", stderr)
			}
		})
	}
}

func TestRunDaemonStatusReportsStoppedTextAndJSON(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", root)

	text := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"status"})
	})
	if !strings.Contains(text, "stopped socket="+filepath.Join(root, "agent-harness.sock")) {
		t.Fatalf("unexpected daemon status text:\n%s", text)
	}

	jsonOut := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"status", "--json"})
	})
	var status daemonStatus
	if err := json.Unmarshal([]byte(jsonOut), &status); err != nil {
		t.Fatalf("decode daemon status JSON: %v\n%s", err, jsonOut)
	}
	if !status.OK || status.Running || status.Paths.Dir != root || status.Message != "daemon is not running" {
		t.Fatalf("unexpected daemon status JSON: %#v", status)
	}
}

func TestRunDaemonStartReportsExistingDaemonTextAndJSON(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "ahd-start-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(root)
	})
	t.Setenv("HARNESS_DAEMON_DIR", root)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", paths.Socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.WriteFile(paths.PID, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	text := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"start"})
	})
	if !strings.Contains(text, "agent-harness daemon running pid="+strconv.Itoa(os.Getpid())) || !strings.Contains(text, "socket="+paths.Socket) {
		t.Fatalf("unexpected daemon start text:\n%s", text)
	}

	jsonOut := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"start", "--json"})
	})
	var status daemonStatus
	if err := json.Unmarshal([]byte(jsonOut), &status); err != nil {
		t.Fatalf("decode daemon start JSON: %v\n%s", err, jsonOut)
	}
	if !status.OK || !status.Running || status.PID != os.Getpid() || status.Paths.Socket != paths.Socket {
		t.Fatalf("unexpected daemon start JSON: %#v", status)
	}
}

func TestRunDaemonStopAlreadyStoppedCleansState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", root)
	socketPath := filepath.Join(root, "agent-harness.sock")
	pidPath := filepath.Join(root, "agent-harness.pid")
	if err := os.WriteFile(socketPath, []byte("stale socket marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("999999\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	text := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"stop"})
	})
	if !strings.Contains(text, "agent-harness daemon already stopped") {
		t.Fatalf("unexpected daemon stop text:\n%s", text)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("stale socket was not removed: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("stale pid was not removed: %v", err)
	}

	jsonOut := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"stop", "--json"})
	})
	var status daemonStatus
	if err := json.Unmarshal([]byte(jsonOut), &status); err != nil {
		t.Fatalf("decode daemon stop JSON: %v\n%s", err, jsonOut)
	}
	if !status.OK || status.Running || status.Message != "agent-harness daemon already stopped" {
		t.Fatalf("unexpected daemon stop JSON: %#v", status)
	}
}

func TestDaemonStatusForMCPUsesCurrentStatus(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", root)

	status := daemonStatusForMCP()

	if !status.OK || status.Running || status.Paths.Dir != root || status.Message != "daemon is not running" {
		t.Fatalf("unexpected daemon MCP status: %#v", status)
	}
}
