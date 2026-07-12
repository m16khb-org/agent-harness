package daemoncli

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDaemonServerDefaultDepsForwardsMCPContext(t *testing.T) {
	oldServe := ServeMCPStreamContext
	t.Cleanup(func() { ServeMCPStreamContext = oldServe })
	type contextKey string
	const key contextKey = "session"
	ctx := context.WithValue(context.Background(), key, "admitted")
	called := false
	ServeMCPStreamContext = func(got context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
		called = true
		if got.Value(key) != "admitted" {
			t.Fatalf("session context was not forwarded: %v", got.Value(key))
		}
		return nil
	}
	conn := &daemonServerFakeConn{closedCh: make(chan struct{}, 1)}
	if err := daemonServerDefaultDeps().serveMCPStream(ctx, conn, &daemonServerFakeLog{}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("context-aware MCP stream handler was not called")
	}
}

func TestRunDaemonServerWithDepsInitializesStateAndExitsOnClosedListener(t *testing.T) {
	dir := t.TempDir()
	paths := daemonPaths{
		Dir:    dir,
		Socket: filepath.Join(dir, "agent-harness.sock"),
		PID:    filepath.Join(dir, "agent-harness.pid"),
		Lock:   filepath.Join(dir, "agent-harness.lock"),
		Log:    filepath.Join(dir, "agent-harness.log"),
	}
	var log daemonServerFakeLog
	removed := []string{}
	var wroteInstance daemonInstance
	tokenCalls := 0

	err := runDaemonServerWithDeps(daemonServerDeps{
		paths: func() (daemonPaths, error) {
			return paths, nil
		},
		mkdirAll: func(path string, perm os.FileMode) error {
			if path != dir || perm != 0o700 {
				t.Fatalf("unexpected mkdir: %s %o", path, perm)
			}
			return nil
		},
		openLog: func(path string) (daemonServerLogFile, error) {
			if path != paths.Log {
				t.Fatalf("unexpected log path: %s", path)
			}
			return &log, nil
		},
		remove: func(path string) error {
			removed = append(removed, path)
			return nil
		},
		listen: func(network, address string) (net.Listener, error) {
			if network != "unix" || address != paths.Socket {
				t.Fatalf("unexpected listen target: %s %s", network, address)
			}
			return daemonServerClosedListener{}, nil
		},
		chmod: func(path string, perm os.FileMode) error {
			if path != paths.Socket || perm != 0o600 {
				t.Fatalf("unexpected chmod: %s %o", path, perm)
			}
			return nil
		},
		writeInstance: func(path string, instance daemonInstance) error {
			if path != paths.PID {
				t.Fatalf("unexpected instance path: %s", path)
			}
			wroteInstance = instance
			return nil
		},
		getpid: func() int {
			return 12345
		},
		inspectProcess: func(pid int) (daemonProcessIdentity, error) {
			if pid != 12345 {
				t.Fatalf("unexpected inspected pid: %d", pid)
			}
			return daemonProcessIdentity{StartTime: "start-a", Executable: "/tmp/agent-harness"}, nil
		},
		buildSHA: func(executable string) (string, error) {
			if executable != "/tmp/agent-harness" {
				t.Fatalf("unexpected executable hash target: %s", executable)
			}
			return "build-a", nil
		},
		newToken: func() (string, error) {
			tokenCalls++
			if tokenCalls == 1 {
				return "nonce-a", nil
			}
			return "generation-a", nil
		},
		now: func() time.Time {
			return time.Unix(100, 0).UTC()
		},
		serveMCPStream: func(context.Context, net.Conn, daemonServerLogFile) error {
			t.Fatal("closed listener should not serve MCP streams")
			return nil
		},
	})

	if err != nil {
		t.Fatalf("expected closed listener to stop cleanly, got %v", err)
	}
	wantInstance := daemonInstance{
		PID:              12345,
		ProcessStartTime: "start-a",
		Executable:       "/tmp/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	if wroteInstance != wantInstance {
		t.Fatalf("unexpected instance write: %#v", wroteInstance)
	}
	if !strings.Contains(log.String(), "daemon started pid=12345 socket="+paths.Socket) {
		t.Fatalf("missing start log: %q", log.String())
	}
	if !containsDaemonServerString(removed, paths.Socket) || !containsDaemonServerString(removed, paths.Lock) || !containsDaemonServerString(removed, paths.PID) {
		t.Fatalf("expected socket/lock/pid cleanup, got %v", removed)
	}
}

func TestRunDaemonServerWithDepsReturnsSetupErrors(t *testing.T) {
	setupErr := errors.New("paths failed")
	err := runDaemonServerWithDeps(daemonServerDeps{
		paths: func() (daemonPaths, error) {
			return daemonPaths{}, setupErr
		},
	})
	if !errors.Is(err, setupErr) {
		t.Fatalf("expected paths error, got %v", err)
	}

	listenErr := errors.New("listen failed")
	err = runDaemonServerWithDeps(daemonServerDeps{
		paths: func() (daemonPaths, error) {
			return daemonPaths{Dir: t.TempDir(), Socket: "daemon.sock", Log: "daemon.log"}, nil
		},
		mkdirAll: func(string, os.FileMode) error { return nil },
		openLog: func(string) (daemonServerLogFile, error) {
			return &daemonServerFakeLog{}, nil
		},
		remove: func(string) error { return nil },
		listen: func(string, string) (net.Listener, error) {
			return nil, listenErr
		},
	})
	if !errors.Is(err, listenErr) {
		t.Fatalf("expected listen error, got %v", err)
	}
}

func TestRunDaemonServerUsesDefaultDepsFactory(t *testing.T) {
	oldFactory := daemonServerDefaultDeps
	t.Cleanup(func() {
		daemonServerDefaultDeps = oldFactory
	})
	called := false
	daemonServerDefaultDeps = func() daemonServerDeps {
		called = true
		return daemonServerDeps{
			paths: func() (daemonPaths, error) {
				return daemonPaths{Dir: t.TempDir(), Socket: "daemon.sock", Log: "daemon.log"}, nil
			},
			mkdirAll: func(string, os.FileMode) error { return nil },
			openLog: func(string) (daemonServerLogFile, error) {
				return &daemonServerFakeLog{}, nil
			},
			remove: func(string) error { return nil },
			listen: func(string, string) (net.Listener, error) {
				return daemonServerClosedListener{}, nil
			},
			chmod:         func(string, os.FileMode) error { return nil },
			writeInstance: func(string, daemonInstance) error { return nil },
			getpid:        func() int { return 12345 },
			inspectProcess: func(int) (daemonProcessIdentity, error) {
				return daemonProcessIdentity{StartTime: "start-a", Executable: "/tmp/agent-harness"}, nil
			},
			buildSHA: func(string) (string, error) { return "build-a", nil },
			newToken: func() (string, error) {
				return "token-a", nil
			},
			now: func() time.Time {
				return time.Unix(100, 0).UTC()
			},
			serveMCPStream: func(context.Context, net.Conn, daemonServerLogFile) error {
				t.Fatal("closed listener should not serve MCP streams")
				return nil
			},
		}
	}

	if err := runDaemonServer(); err != nil {
		t.Fatalf("expected wrapper to stop cleanly, got %v", err)
	}
	if !called {
		t.Fatal("default deps factory was not called")
	}
}
