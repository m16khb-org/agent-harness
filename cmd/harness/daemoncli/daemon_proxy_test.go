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
	"reflect"
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
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
		},
		dial: func(_ context.Context, network, address string) (io.ReadWriteCloser, error) {
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

func TestRunMCPProxyWithDepsReconnectsWithoutReplayingInterruptedRequest(t *testing.T) {
	firstProxy, firstDaemon := net.Pipe()
	secondProxy, secondDaemon := net.Pipe()
	t.Cleanup(func() {
		_ = firstProxy.Close()
		_ = firstDaemon.Close()
		_ = secondProxy.Close()
		_ = secondDaemon.Close()
	})
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	t.Cleanup(func() {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	})

	var dialMu sync.Mutex
	dialCount := 0
	transientDialErr := errors.New("daemon socket is being replaced")
	done := make(chan error, 1)
	go func() {
		done <- runMCPProxyWithDeps(daemonProxyDeps{
			ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
				return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
			},
			dial: func(context.Context, string, string) (io.ReadWriteCloser, error) {
				dialMu.Lock()
				defer dialMu.Unlock()
				dialCount++
				switch dialCount {
				case 1:
					return firstProxy, nil
				case 2:
					return nil, transientDialErr
				case 3:
					return secondProxy, nil
				default:
					return nil, errors.New("unexpected extra daemon dial")
				}
			},
			stdin:  stdinReader,
			stdout: stdoutWriter,
		})
	}()

	firstDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(firstDaemon)
		initialize, err := reader.ReadString('\n')
		if err != nil {
			firstDone <- err
			return
		}
		if method := daemonProxyTestMethod(initialize); method != "initialize" {
			firstDone <- errors.New("first daemon did not receive initialize")
			return
		}
		if _, err := io.WriteString(firstDaemon, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"resources":{"listChanged":true},"tools":{"listChanged":true}}}}`+"\n"); err != nil {
			firstDone <- err
			return
		}
		initialized, err := reader.ReadString('\n')
		if err != nil {
			firstDone <- err
			return
		}
		if method := daemonProxyTestMethod(initialized); method != "notifications/initialized" {
			firstDone <- errors.New("first daemon did not receive initialized notification")
			return
		}
		interrupted, err := reader.ReadString('\n')
		if err != nil {
			firstDone <- err
			return
		}
		if id := daemonProxyTestID(interrupted); id != "2" {
			firstDone <- errors.New("first daemon did not receive request 2")
			return
		}
		firstDone <- firstDaemon.Close()
	}()

	secondReady := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(secondDaemon)
		initialize, err := reader.ReadString('\n')
		if err != nil {
			secondReady <- err
			return
		}
		if method := daemonProxyTestMethod(initialize); method != "initialize" {
			secondReady <- errors.New("second daemon did not receive replayed initialize")
			return
		}
		if _, err := io.WriteString(secondDaemon, `{"jsonrpc":"2.0","id":"agent-harness-reconnect-1","result":{"protocolVersion":"2025-06-18","capabilities":{"resources":{"listChanged":true},"tools":{"listChanged":true}}}}`+"\n"); err != nil {
			secondReady <- err
			return
		}
		initialized, err := reader.ReadString('\n')
		if err != nil {
			secondReady <- err
			return
		}
		if method := daemonProxyTestMethod(initialized); method != "notifications/initialized" {
			secondReady <- errors.New("second daemon did not receive replayed initialized notification")
			return
		}
		secondReady <- nil

		next, err := reader.ReadString('\n')
		if err != nil {
			secondDone <- err
			return
		}
		if id := daemonProxyTestID(next); id != "3" {
			secondDone <- errors.New("interrupted request was replayed to second daemon")
			return
		}
		if _, err := io.WriteString(secondDaemon, `{"jsonrpc":"2.0","id":3,"result":{"ok":true}}`+"\n"); err != nil {
			secondDone <- err
			return
		}
		secondDone <- nil
	}()

	outputLines := make(chan string, 8)
	outputErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			outputLines <- scanner.Text()
		}
		outputErr <- scanner.Err()
	}()

	for _, line := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"state_write","arguments":{"key":"k","value":"v"}}}`,
	} {
		if _, err := io.WriteString(stdinWriter, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}

	if err := daemonProxyTestReceiveError(t, firstDone); err != nil {
		t.Fatal(err)
	}
	initializeResponse := daemonProxyTestReceiveLine(t, outputLines)
	if id := daemonProxyTestID(initializeResponse); id != "1" {
		t.Fatalf("initialize response id = %q, want 1: %s", id, initializeResponse)
	}
	interruptedResponse := daemonProxyTestReceiveLine(t, outputLines)
	var interrupted struct {
		ID    json.RawMessage `json:"id"`
		Error struct {
			Code int `json:"code"`
			Data struct {
				Code              string `json:"code"`
				Outcome           string `json:"outcome"`
				AutomaticRetry    bool   `json:"automatic_retry"`
				ReconcileRequired bool   `json:"reconcile_required"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(interruptedResponse), &interrupted); err != nil {
		t.Fatalf("decode interrupted response: %v\n%s", err, interruptedResponse)
	}
	if string(interrupted.ID) != "2" || interrupted.Error.Code != -32002 ||
		interrupted.Error.Data.Code != "daemon_generation_changed" ||
		interrupted.Error.Data.Outcome != "unknown" ||
		interrupted.Error.Data.AutomaticRetry ||
		!interrupted.Error.Data.ReconcileRequired {
		t.Fatalf("unexpected interrupted response: %#v", interrupted)
	}
	if err := daemonProxyTestReceiveError(t, secondReady); err != nil {
		t.Fatal(err)
	}

	gotNotifications := []string{
		daemonProxyTestMethod(daemonProxyTestReceiveLine(t, outputLines)),
		daemonProxyTestMethod(daemonProxyTestReceiveLine(t, outputLines)),
	}
	wantNotifications := []string{"notifications/tools/list_changed", "notifications/resources/list_changed"}
	if !reflect.DeepEqual(gotNotifications, wantNotifications) {
		t.Fatalf("reconnect notifications = %#v, want %#v", gotNotifications, wantNotifications)
	}
	if _, err := io.WriteString(stdinWriter, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"daemon_status","arguments":{}}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	nextResponse := daemonProxyTestReceiveLine(t, outputLines)
	if id := daemonProxyTestID(nextResponse); id != "3" {
		t.Fatalf("next response id = %q, want 3: %s", id, nextResponse)
	}
	if err := daemonProxyTestReceiveError(t, secondDone); err != nil {
		t.Fatal(err)
	}

	if err := stdinWriter.Close(); err != nil {
		t.Fatal(err)
	}
	_ = secondDaemon.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("proxy returned error after host EOF: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("proxy did not stop after host EOF")
	}
	_ = stdoutWriter.Close()
	select {
	case err := <-outputErr:
		if err != nil {
			t.Fatalf("stdout reader error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stdout reader did not stop")
	}
}

func TestRunMCPProxyWithDepsReturnsSetupAndDialErrors(t *testing.T) {
	setupErr := errors.New("daemon unavailable")
	err := runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			return daemonStatus{}, setupErr
		},
	})
	if !errors.Is(err, setupErr) {
		t.Fatalf("expected setup error, got %v", err)
	}

	dialErr := errors.New("socket missing")
	err = runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
		},
		dial: func(context.Context, string, string) (io.ReadWriteCloser, error) {
			return nil, dialErr
		},
	})
	if err == nil || !strings.Contains(err.Error(), "connect daemon") || !errors.Is(err, dialErr) {
		t.Fatalf("expected wrapped dial error, got %v", err)
	}
}

func TestRunMCPProxyRejectsSaturatedDaemonBeforeDialWithoutStdout(t *testing.T) {
	var stdout bytes.Buffer
	dialed := false
	err := runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			return daemonStatus{
				ActiveConnections: maxConnections,
				MaxConnections:    maxConnections,
				Accepting:         false,
				Paths:             daemonPaths{Socket: "daemon.sock"},
			}, nil
		},
		dial: func(context.Context, string, string) (io.ReadWriteCloser, error) {
			dialed = true
			return nil, errors.New("saturated daemon must not be dialed")
		},
		stdin:  strings.NewReader("initialize\n"),
		stdout: &stdout,
	})

	if err == nil || !strings.Contains(err.Error(), daemonStatusConnectionLimit) {
		t.Fatalf("expected explicit saturation error, got %v", err)
	}
	if dialed {
		t.Fatal("saturated proxy must reject before dialing")
	}
	if stdout.Len() != 0 {
		t.Fatalf("saturated proxy polluted MCP stdout: %q", stdout.String())
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
	instance := writeVerifiedDaemonTestInstance(t, paths)
	proxyDone := make(chan string, 1)
	serverDone := make(chan string, 1)
	go serveDaemonProxyTestSocket(t, listener, instance, serverDone)

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
	type readResult struct {
		out []byte
		err error
	}
	outDone := make(chan readResult, 1)
	go func() {
		out, err := io.ReadAll(outR)
		outDone <- readResult{out: out, err: err}
	}()
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
	read := <-outDone
	if read.err != nil {
		t.Fatal(read.err)
	}
	if string(read.out) != "daemon response\n" {
		t.Fatalf("unexpected proxy stdout: %q", string(read.out))
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
