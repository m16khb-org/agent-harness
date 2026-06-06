package daemoncli

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunMCPProxyWithDepsCopiesDaemonAndStdinStreams(t *testing.T) {
	conn := &daemonProxyFakeConn{
		reader: strings.NewReader("daemon response\n"),
	}
	var stdout bytes.Buffer

	err := runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: func() (daemonStatus, error) {
			return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
		},
		dial: func(network, address string) (io.ReadWriteCloser, error) {
			if network != "unix" || address != "daemon.sock" {
				t.Fatalf("unexpected dial target: %s %s", network, address)
			}
			return conn, nil
		},
		stdin:  strings.NewReader("client request\n"),
		stdout: &stdout,
	})

	if err != nil {
		t.Fatalf("expected proxy copy success, got %v", err)
	}
	if stdout.String() != "daemon response\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	conn.waitForWrite(t)
	if got, closed := conn.writerString(), conn.isClosed(); got != "client request\n" || !closed {
		t.Fatalf("unexpected daemon write/close state: write=%q closed=%v", got, closed)
	}
}

func TestRunMCPProxyWithDepsReturns_whenDaemonClosesBeforeStdin(t *testing.T) {
	stdin := newDaemonProxyBlockingReader()
	conn := &daemonProxyFakeConn{
		reader: strings.NewReader("daemon response\n"),
	}
	var stdout bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- runMCPProxyWithDeps(daemonProxyDeps{
			ensureDaemonRunning: func() (daemonStatus, error) {
				return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
			},
			dial: func(string, string) (io.ReadWriteCloser, error) {
				return conn, nil
			},
			stdin:  stdin,
			stdout: &stdout,
		})
	}()

	select {
	case err := <-done:
		stdin.release()
		if err != nil {
			t.Fatalf("expected proxy to return without error, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		stdin.release()
		err := <-done
		t.Fatalf("runMCPProxyWithDeps waited for stdin after daemon closed: %v", err)
	}
	if stdout.String() != "daemon response\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestRunMCPProxyWithDepsReturnsSetupAndDialErrors(t *testing.T) {
	setupErr := errors.New("daemon unavailable")
	err := runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: func() (daemonStatus, error) {
			return daemonStatus{}, setupErr
		},
	})
	if !errors.Is(err, setupErr) {
		t.Fatalf("expected setup error, got %v", err)
	}

	dialErr := errors.New("socket missing")
	err = runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: func() (daemonStatus, error) {
			return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
		},
		dial: func(string, string) (io.ReadWriteCloser, error) {
			return nil, dialErr
		},
	})
	if err == nil || !strings.Contains(err.Error(), "connect daemon") || !errors.Is(err, dialErr) {
		t.Fatalf("expected wrapped dial error, got %v", err)
	}
}

func TestRunMCPProxyUsesExistingDaemonSocket(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "ahd-proxy-")
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
	proxyDone := make(chan string, 1)
	serverDone := make(chan string, 1)
	go serveDaemonProxyTestSocket(t, listener, serverDone)

	oldStdin, oldStdout := os.Stdin, os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = inR
	os.Stdout = outW
	if _, err := inW.WriteString("client request\n"); err != nil {
		t.Fatal(err)
	}
	if err := inW.Close(); err != nil {
		t.Fatal(err)
	}
	go func() {
		proxyDone <- runMCPProxyErrorString()
	}()

	select {
	case got := <-proxyDone:
		_ = outW.Close()
		if got != "" {
			t.Fatalf("runMCPProxy returned error: %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runMCPProxy did not return")
	}
	out, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "daemon response\n" {
		t.Fatalf("unexpected proxy stdout: %q", string(out))
	}
	select {
	case got := <-serverDone:
		if got != "client request\n" {
			t.Fatalf("unexpected daemon request: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon proxy test server did not receive request")
	}
}

func runMCPProxyErrorString() string {
	if err := runMCPProxy(); err != nil {
		return err.Error()
	}
	return ""
}

func serveDaemonProxyTestSocket(t *testing.T, listener net.Listener, serverDone chan<- string) {
	t.Helper()
	statusConn, err := listener.Accept()
	if err != nil {
		serverDone <- "accept status: " + err.Error()
		return
	}
	_ = statusConn.Close()
	proxyConn, err := listener.Accept()
	if err != nil {
		serverDone <- "accept proxy: " + err.Error()
		return
	}
	defer proxyConn.Close()
	if _, err := proxyConn.Write([]byte("daemon response\n")); err != nil {
		serverDone <- "write response: " + err.Error()
		return
	}
	request, err := io.ReadAll(proxyConn)
	if err != nil {
		serverDone <- "read request: " + err.Error()
		return
	}
	serverDone <- string(request)
}

type daemonProxyFakeConn struct {
	mu        sync.Mutex
	reader    io.Reader
	writer    bytes.Buffer
	closed    bool
	writeDone chan struct{}
}

func (c *daemonProxyFakeConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *daemonProxyFakeConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	n, err := c.writer.Write(p)
	if c.writeDone == nil {
		c.writeDone = make(chan struct{})
	}
	select {
	case <-c.writeDone:
	default:
		close(c.writeDone)
	}
	c.mu.Unlock()
	return n, err
}

func (c *daemonProxyFakeConn) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *daemonProxyFakeConn) writerString() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.String()
}

func (c *daemonProxyFakeConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *daemonProxyFakeConn) waitForWrite(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	if c.writeDone == nil {
		c.writeDone = make(chan struct{})
	}
	writeDone := c.writeDone
	c.mu.Unlock()
	select {
	case <-writeDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("daemon proxy fake connection did not receive stdin copy")
	}
}

type daemonProxyBlockingReader struct {
	released chan struct{}
}

func newDaemonProxyBlockingReader() *daemonProxyBlockingReader {
	return &daemonProxyBlockingReader{released: make(chan struct{})}
}

func (r *daemonProxyBlockingReader) Read([]byte) (int, error) {
	<-r.released
	return 0, io.EOF
}

func (r *daemonProxyBlockingReader) release() {
	close(r.released)
}
