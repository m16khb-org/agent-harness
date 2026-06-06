package daemoncli

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	writes := map[string]string{}

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
		writeFile: func(path string, content []byte, perm os.FileMode) error {
			if perm != 0o600 {
				t.Fatalf("unexpected write perm: %o", perm)
			}
			writes[path] = string(content)
			return nil
		},
		getpid: func() int {
			return 12345
		},
		now: func() time.Time {
			return time.Unix(100, 0).UTC()
		},
		serveMCPStream: func(net.Conn, daemonServerLogFile) error {
			t.Fatal("closed listener should not serve MCP streams")
			return nil
		},
	})

	if err != nil {
		t.Fatalf("expected closed listener to stop cleanly, got %v", err)
	}
	if writes[paths.PID] != "12345\n" {
		t.Fatalf("unexpected pid write: %q", writes[paths.PID])
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
			chmod:     func(string, os.FileMode) error { return nil },
			writeFile: func(string, []byte, os.FileMode) error { return nil },
			getpid:    func() int { return 12345 },
			now: func() time.Time {
				return time.Unix(100, 0).UTC()
			},
			serveMCPStream: func(net.Conn, daemonServerLogFile) error {
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

func TestRunDaemonAcceptLoopLogsAcceptAndStreamErrors(t *testing.T) {
	var log daemonServerFakeLog
	now := time.Unix(200, 0).UTC()
	conn := &daemonServerFakeConn{closedCh: make(chan struct{}, 1)}
	listener := &daemonServerScriptedListener{
		accepts: []daemonServerAccept{
			{err: errors.New("temporary accept failure")},
			{conn: conn},
			{err: errors.New("use of closed network connection")},
		},
	}

	err := runDaemonAcceptLoop(listener, &log, daemonServerLoopDeps{
		now: func() time.Time {
			return now
		},
		serveMCPStream: func(net.Conn, daemonServerLogFile) error {
			return errors.New("stream failed")
		},
	})

	if err != nil {
		t.Fatalf("expected closed listener to stop loop, got %v", err)
	}
	select {
	case <-conn.closedCh:
	case <-time.After(time.Second):
		t.Fatal("serveMCPStream goroutine did not finish")
	}
	text := log.String()
	if !strings.Contains(text, "temporary accept failure") || !strings.Contains(text, "mcp stream error: stream failed") {
		t.Fatalf("missing loop diagnostics: %q", text)
	}
}

type daemonServerFakeLog struct {
	bytes.Buffer
}

func (l *daemonServerFakeLog) Close() error {
	return nil
}

type daemonServerClosedListener struct{}

func (daemonServerClosedListener) Accept() (net.Conn, error) {
	return nil, errors.New("use of closed network connection")
}

func (daemonServerClosedListener) Close() error {
	return nil
}

func (daemonServerClosedListener) Addr() net.Addr {
	return nil
}

type daemonServerAccept struct {
	conn net.Conn
	err  error
}

type daemonServerScriptedListener struct {
	accepts []daemonServerAccept
}

func (l *daemonServerScriptedListener) Accept() (net.Conn, error) {
	if len(l.accepts) == 0 {
		return nil, errors.New("use of closed network connection")
	}
	next := l.accepts[0]
	l.accepts = l.accepts[1:]
	return next.conn, next.err
}

func (l *daemonServerScriptedListener) Close() error {
	return nil
}

func (l *daemonServerScriptedListener) Addr() net.Addr {
	return nil
}

type daemonServerFakeConn struct {
	closed   bool
	closedCh chan struct{}
}

func (c *daemonServerFakeConn) Read([]byte) (int, error) {
	return 0, errors.New("not implemented")
}

func (c *daemonServerFakeConn) Write(p []byte) (int, error) {
	return len(p), nil
}

func (c *daemonServerFakeConn) Close() error {
	c.closed = true
	if c.closedCh != nil {
		c.closedCh <- struct{}{}
	}
	return nil
}

func (c *daemonServerFakeConn) LocalAddr() net.Addr {
	return nil
}

func (c *daemonServerFakeConn) RemoteAddr() net.Addr {
	return nil
}

func (c *daemonServerFakeConn) SetDeadline(time.Time) error {
	return nil
}

func (c *daemonServerFakeConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *daemonServerFakeConn) SetWriteDeadline(time.Time) error {
	return nil
}

func containsDaemonServerString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
