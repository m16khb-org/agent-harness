package daemoncli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func runMCPProxyErrorString() string {
	if err := runMCPProxy(); err != nil {
		return err.Error()
	}
	return ""
}

func serveDaemonProxyTestSocket(t *testing.T, listener net.Listener, instance daemonInstance, serverDone chan<- string) {
	t.Helper()
	statusConn, err := listener.Accept()
	if err != nil {
		serverDone <- "accept status: " + err.Error()
		return
	}
	if err := serveDaemonConnection(statusConn, &daemonServerFakeLog{}, instance, func(net.Conn, daemonServerLogFile) error {
		return errors.New("status connection must use identity probe")
	}); err != nil {
		_ = statusConn.Close()
		serverDone <- "serve status: " + err.Error()
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

func daemonProxyTestMethod(line string) string {
	var envelope struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal([]byte(line), &envelope)
	return envelope.Method
}

func daemonProxyTestID(line string) string {
	var envelope struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal([]byte(line), &envelope)
	return string(envelope.ID)
}

func daemonProxyTestReceiveLine(t *testing.T, lines <-chan string) string {
	t.Helper()
	select {
	case line := <-lines:
		return line
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for proxy output")
		return ""
	}
}

func daemonProxyTestReceiveError(t *testing.T, errors <-chan error) error {
	t.Helper()
	select {
	case err := <-errors:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for proxy test stage")
		return nil
	}
}
