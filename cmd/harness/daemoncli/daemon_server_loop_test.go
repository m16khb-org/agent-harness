package daemoncli

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"sync"
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

	var wg sync.WaitGroup
	connSlots := make(chan struct{}, maxConnections)
	err := runDaemonAcceptLoop(listener, &log, daemonServerLoopDeps{
		now: func() time.Time {
			return now
		},
		serveMCPStream: func(net.Conn, daemonServerLogFile) error {
			return errors.New("stream failed")
		},
		connSlots: connSlots,
		activeWG:  &wg,
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
	blocking bool
}

func (c *daemonServerFakeConn) Read([]byte) (int, error) {
	if c.blocking {
		<-c.closedCh
	}
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

func TestRunDaemonAcceptLoopRejectsWhenConnectionLimitReached(t *testing.T) {
	var log daemonServerFakeLog
	now := time.Unix(300, 0).UTC()

	// Create maxConnections+1 accepts: the first maxConnections succeed,
	// the next one should be rejected.
	accepts := make([]daemonServerAccept, 0, maxConnections+2)
	for i := 0; i < maxConnections; i++ {
		accepts = append(accepts, daemonServerAccept{
			conn: &daemonServerFakeConn{closedCh: make(chan struct{}, 1), blocking: true},
		})
	}
	rejectedConn := &daemonServerFakeConn{closedCh: make(chan struct{}, 1), blocking: true}
	accepts = append(accepts, daemonServerAccept{conn: rejectedConn})
	accepts = append(accepts, daemonServerAccept{err: errors.New("use of closed network connection")})

	listener := &daemonServerScriptedListener{accepts: accepts}

	var wg sync.WaitGroup
	connSlots := make(chan struct{}, maxConnections)
	err := runDaemonAcceptLoop(listener, &log, daemonServerLoopDeps{
		now: func() time.Time { return now },
		serveMCPStream: func(c net.Conn, lf daemonServerLogFile) error {
			// block until connection is closed from outside
			<-c.(*daemonServerFakeConn).closedCh
			return nil
		},
		connSlots: connSlots,
		activeWG:  &wg,
	})

	if err != nil {
		t.Fatalf("expected closed listener to stop loop, got %v", err)
	}
	text := log.String()
	if !strings.Contains(text, "connection limit reached") {
		t.Fatalf("expected connection limit rejection in log: %q", text)
	}
	// Verify rejected conn was closed
	if !rejectedConn.closed {
		t.Fatal("rejected connection should have been closed")
	}
}

func TestRunDaemonAcceptLoopGracefulShutdownWaitsForActiveConnections(t *testing.T) {
	var log daemonServerFakeLog
	now := time.Unix(400, 0).UTC()
	conn := &daemonServerFakeConn{closedCh: make(chan struct{}, 1), blocking: true}

	listener := &daemonServerScriptedListener{
		accepts: []daemonServerAccept{
			{conn: conn},
			{err: errors.New("use of closed network connection")},
		},
	}

	var wg sync.WaitGroup
	connSlots := make(chan struct{}, maxConnections)

	// The serveMCPStream blocks until we signal it
	streamStarted := make(chan struct{})
	err := runDaemonAcceptLoop(listener, &log, daemonServerLoopDeps{
		now: func() time.Time { return now },
		serveMCPStream: func(c net.Conn, lf daemonServerLogFile) error {
			streamStarted <- struct{}{}
			// block until closed
			<-c.(*daemonServerFakeConn).closedCh
			return nil
		},
		connSlots: connSlots,
		activeWG:  &wg,
	})

	if err != nil {
		t.Fatalf("expected clean shutdown, got %v", err)
	}

	// The goroutine should have started; now close the conn so it finishes
	select {
	case <-streamStarted:
		// good, the stream started
	case <-time.After(time.Second):
		t.Fatal("stream did not start in time")
	}
	_ = conn.Close()

	// WaitGroup should eventually reach 0 (from the goroutine finishing)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// graceful shutdown worked
	case <-time.After(5 * time.Second):
		t.Fatal("WaitGroup did not drain within timeout")
	}
}
