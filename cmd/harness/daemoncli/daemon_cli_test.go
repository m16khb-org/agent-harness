package daemoncli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
			stderr, err := captureProjectCLIStderr(t, func() error {
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
	instance := startVerifiedDaemonTestSocket(t, paths)

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
	if !status.OK || !status.Running || !status.Reachable || !status.IdentityVerified || status.PID != os.Getpid() || status.Paths.Socket != paths.Socket || status.Instance == nil || *status.Instance != instance {
		t.Fatalf("unexpected daemon start JSON: %#v", status)
	}
}

func TestRunDaemonStopReportsAlreadyStopped(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", root)

	text := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"stop"})
	})
	if !strings.Contains(text, "agent-harness daemon already stopped") {
		t.Fatalf("unexpected daemon stop text:\n%s", text)
	}
	jsonOut := captureStatusVerifyStdout(t, func() error {
		return runDaemon([]string{"stop", "--json"})
	})
	var status daemonStatus
	if err := json.Unmarshal([]byte(jsonOut), &status); err != nil {
		t.Fatalf("decode daemon stop JSON: %v\n%s", err, jsonOut)
	}
	if !status.OK || status.Running || status.Code != daemonStatusStopped || status.Message != "agent-harness daemon already stopped" {
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

func TestDaemonStopCleansStalePIDFileIdempotently(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(root, "agent-harness.pid")
	// Stale state: pid file references a dead PID, socket absent.
	if err := os.WriteFile(pidPath, []byte(`{"pid":99999,"process_start_time":"2026-08-20T13:27:20Z","executable":"/bin/agent-harness","instance_nonce":"n","build_sha":"s","protocol_version":"1","generation":"g"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	removed := map[string]bool{}
	status, err := stopDaemonWithDeps(daemonStopDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{
				OK:    true,
				Code:  daemonStatusStopped,
				PID:   99999,
				Paths: daemonPaths{Dir: root, Socket: filepath.Join(root, "agent-harness.sock"), PID: pidPath},
			}
		},
		findProcess: func(int) (daemonProcess, error) { return nil, fmt.Errorf("unused") },
		inspectProcess: func(int) (daemonProcessIdentity, error) { return daemonProcessIdentity{}, fmt.Errorf("unused") },
		processAlive: func(pid int) bool { return false },
		remove:       func(path string) error { removed[path] = true; return nil },
		now:          time.Now,
		sleep:        func(time.Duration) {},
	})
	if err != nil {
		t.Fatalf("stop with stale pid file must be idempotent: %v", err)
	}
	if !status.OK || status.Code != daemonStatusStopped || status.Running {
		t.Fatalf("unexpected status: %#v", status)
	}
	if !removed[pidPath] {
		t.Fatal("stale pid file must be removed")
	}
}
