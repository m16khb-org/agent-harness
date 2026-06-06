package daemoncli

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

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
