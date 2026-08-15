package daemoncli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type daemonServerDiscardLog struct{ io.Writer }

func (daemonServerDiscardLog) Close() error { return nil }

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
	err := runDaemonAcceptLoop(listener, &log, daemonServerLoopDeps{
		now: func() time.Time {
			return now
		},
		serveConnection: func(net.Conn, daemonServerLogFile) error {
			return errors.New("stream failed")
		},
		activeWG: &wg,
	})

	if err != nil {
		t.Fatalf("expected closed listener to stop loop, got %v", err)
	}
	select {
	case <-conn.closedCh:
	case <-time.After(time.Second):
		t.Fatal("serveConnection goroutine did not finish")
	}
	text := log.String()
	if !strings.Contains(text, "temporary accept failure") || !strings.Contains(text, "connection error: stream failed") {
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
	reader   io.Reader
	writes   bytes.Buffer
}

func (c *daemonServerFakeConn) Read(p []byte) (int, error) {
	if c.reader != nil {
		return c.reader.Read(p)
	}
	if c.blocking {
		<-c.closedCh
	}
	return 0, errors.New("not implemented")
}

func (c *daemonServerFakeConn) Write(p []byte) (int, error) {
	return c.writes.Write(p)
}

func (c *daemonServerFakeConn) writtenString() string {
	return c.writes.String()
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
	admission := newDaemonAdmission(1)
	held, ok := admission.acquire()
	if !ok {
		t.Fatal("failed to occupy admission slot")
	}
	defer held.close()
	rejectedConn := &daemonServerFakeConn{
		closedCh: make(chan struct{}, 1),
		reader:   strings.NewReader("{"),
	}
	listener := &daemonServerScriptedListener{accepts: []daemonServerAccept{
		{conn: rejectedConn},
		{err: errors.New("use of closed network connection")},
	}}

	var wg sync.WaitGroup
	err := runDaemonAcceptLoop(listener, &log, daemonServerLoopDeps{
		now: func() time.Time { return now },
		serveConnection: func(conn net.Conn, logFile daemonServerLogFile) error {
			return serveDaemonConnectionWithAdmission(conn, logFile, daemonInstance{}, admission, func(context.Context, net.Conn, daemonServerLogFile) error {
				return errors.New("saturated stream must not start")
			})
		},
		activeWG: &wg,
	})

	if err != nil {
		t.Fatalf("expected closed listener to stop loop, got %v", err)
	}
	select {
	case <-rejectedConn.closedCh:
	case <-time.After(time.Second):
		t.Fatal("rejected connection was not closed")
	}
	wg.Wait()
	if !strings.Contains(log.String(), daemonStatusConnectionLimit) {
		t.Fatalf("expected connection limit rejection in log: %q", log.String())
	}
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		Error   struct {
			Code int            `json:"code"`
			Data map[string]any `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(rejectedConn.writtenString()), &response); err != nil {
		t.Fatalf("connection rejection must be valid JSON-RPC, got %q: %v", rejectedConn.writtenString(), err)
	}
	if response.JSONRPC != "2.0" || response.Error.Code != -32001 {
		t.Fatalf("unexpected admission error: %#v", response)
	}
	if response.Error.Data["code"] != "daemon_connection_limit_reached" || response.Error.Data["accepting"] != false {
		t.Fatalf("admission error lost structured health: %#v", response.Error.Data)
	}
}

func TestDaemonMaxConnectionsBoundsEnvironmentOverride(t *testing.T) {
	tests := []struct {
		value string
		want  int
	}{
		{value: "", want: defaultMaxConnections},
		{value: "512", want: 512},
		{value: "0", want: defaultMaxConnections},
		{value: "4097", want: defaultMaxConnections},
		{value: "invalid", want: defaultMaxConnections},
	}
	for _, test := range tests {
		if got := daemonMaxConnections(test.value); got != test.want {
			t.Fatalf("daemonMaxConnections(%q)=%d want=%d", test.value, got, test.want)
		}
	}
	if defaultMaxConnections < 256 {
		t.Fatalf("default daemon capacity must support multi-session hosts: %d", defaultMaxConnections)
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

	// 연결 핸들러는 우리가 신호를 줄 때까지 블록된다.
	streamStarted := make(chan struct{})
	err := runDaemonAcceptLoop(listener, &log, daemonServerLoopDeps{
		now: func() time.Time { return now },
		serveConnection: func(c net.Conn, _ daemonServerLogFile) error {
			streamStarted <- struct{}{}
			<-c.(*daemonServerFakeConn).closedCh
			return nil
		},
		activeWG: &wg,
	})

	if err != nil {
		t.Fatalf("expected clean shutdown, got %v", err)
	}

	// goroutine이 시작됐을 것이다. 이제 conn을 닫아 종료시킨다.
	select {
	case <-streamStarted:
	case <-time.After(time.Second):
		t.Fatal("stream did not start in time")
	}
	_ = conn.Close()

	// WaitGroup은 goroutine이 종료되면서 결국 0에 도달해야 한다.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitGroup did not drain within timeout")
	}
}

func TestRunDaemonAcceptLoopHealthProbeBypassesFullMCPAdmission(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ahd-admission-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "daemon.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	logFile := daemonServerDiscardLog{Writer: io.Discard}
	started := make(chan struct{}, maxConnections)
	admission := newDaemonAdmission(maxConnections)
	var wg sync.WaitGroup
	loopDone := make(chan error, 1)
	go func() {
		loopDone <- runDaemonAcceptLoop(listener, logFile, daemonServerLoopDeps{
			now: func() time.Time { return time.Now().UTC() },
			serveConnection: func(conn net.Conn, logFile daemonServerLogFile) error {
				return serveDaemonConnectionWithAdmission(conn, logFile, daemonInstance{}, admission, func(_ context.Context, conn net.Conn, _ daemonServerLogFile) error {
					started <- struct{}{}
					_, err := io.Copy(io.Discard, conn)
					return err
				})
			},
			wrapConn: func(conn net.Conn) net.Conn {
				return &idleConn{Conn: conn, timeout: time.Minute}
			},
			activeWG: &wg,
		})
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-loopDone:
		case <-time.After(time.Second):
		}
	})

	clients := make([]net.Conn, 0, maxConnections)
	for i := 0; i < maxConnections; i++ {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, conn)
		if _, err := io.WriteString(conn, "{"); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, conn := range clients {
			_ = conn.Close()
		}
	})
	for i := 0; i < maxConnections; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d/%d MCP sessions started", i, maxConnections)
		}
	}

	statusConn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer statusConn.Close()
	if err := statusConn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(statusConn, daemonIdentityRequest); err != nil {
		t.Fatal(err)
	}
	var response daemonIdentityResponse
	if err := json.NewDecoder(statusConn).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.ActiveConnections != maxConnections || response.Accepting {
		t.Fatalf("health probe did not bypass saturated MCP admission: %#v", response)
	}
}

func TestRunDaemonAcceptLoopExpires64IdleSessionsAndAdmitsInitialize(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ahd-idle-expiry-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "daemon.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	logFile := daemonServerDiscardLog{Writer: io.Discard}
	admission := newDaemonAdmission(maxConnections)
	requests := make(chan string, maxConnections+1)
	started := make(chan struct{}, maxConnections+1)
	completed := make(chan error, maxConnections+1)
	var wg sync.WaitGroup
	loopDone := make(chan error, 1)
	go func() {
		loopDone <- runDaemonAcceptLoop(listener, logFile, daemonServerLoopDeps{
			now: func() time.Time { return time.Now().UTC() },
			serveConnection: func(conn net.Conn, logFile daemonServerLogFile) error {
				serveErr := serveDaemonConnectionWithAdmission(conn, logFile, daemonInstance{}, admission, func(_ context.Context, conn net.Conn, _ daemonServerLogFile) error {
					started <- struct{}{}
					request, err := bufio.NewReader(conn).ReadString('\n')
					if err == nil {
						requests <- request
					}
					return err
				})
				completed <- serveErr
				return serveErr
			},
			wrapConn: func(conn net.Conn) net.Conn {
				return &idleConn{Conn: conn, timeout: 2 * time.Second}
			},
			activeWG: &wg,
		})
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-loopDone:
		case <-time.After(time.Second):
		}
	})

	clients := make([]net.Conn, 0, maxConnections)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < maxConnections; i++ {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, conn)
		if _, err := io.WriteString(conn, "{"); err != nil {
			t.Fatal(err)
		}
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatalf("only %d/%d idle sessions reached MCP dispatch: %v", i, maxConnections, ctx.Err())
		}
	}
	t.Cleanup(func() {
		for _, conn := range clients {
			_ = conn.Close()
		}
	})
	if got := admission.snapshot(); got.ActiveConnections != maxConnections || got.Accepting {
		t.Fatalf("expected saturated admission before expiry, got %#v", got)
	}

	for i := 0; i < maxConnections; i++ {
		select {
		case <-completed:
		case <-ctx.Done():
			t.Fatalf("only %d/%d idle sessions expired: %v", i, maxConnections, ctx.Err())
		}
	}
	if got := admission.snapshot(); got.ActiveConnections != 0 || !got.Accepting {
		t.Fatalf("idle expiry did not release every slot: %#v", got)
	}

	const initialize = "{\"jsonrpc\":\"2.0\",\"id\":65,\"method\":\"initialize\"}\n"
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, initialize); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-requests:
		if got != initialize {
			t.Fatalf("initialize was not replayed exactly: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("initialize was not admitted after idle expiry")
	}
}
