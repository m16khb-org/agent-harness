package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
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
	if conn.writer.String() != "client request\n" || !conn.closed {
		t.Fatalf("unexpected daemon write/close state: write=%q closed=%v", conn.writer.String(), conn.closed)
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

type daemonProxyFakeConn struct {
	reader io.Reader
	writer bytes.Buffer
	closed bool
}

func (c *daemonProxyFakeConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *daemonProxyFakeConn) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

func (c *daemonProxyFakeConn) Close() error {
	c.closed = true
	return nil
}
