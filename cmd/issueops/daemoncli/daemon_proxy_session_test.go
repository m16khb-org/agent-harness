package daemoncli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDaemonProxySessionFailsPendingBatchAsBatch(t *testing.T) {
	session := newDaemonProxySession()
	session.observeHost([]byte(`[
		{"jsonrpc":"2.0","id":"a\u0062","method":"tools/call","params":{"name":"state_write"}},
		{"jsonrpc":"2.0","method":"notifications/progress","params":{}},
		{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"state_write"}}
	]` + "\n"))

	var stdout bytes.Buffer
	if err := session.failPending(&stdout); err != nil {
		t.Fatal(err)
	}
	var responses []struct {
		ID    json.RawMessage `json:"id"`
		Error struct {
			Code int `json:"code"`
			Data struct {
				Outcome string `json:"outcome"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &responses); err != nil {
		t.Fatalf("batch unknown response decode: %v\n%s", err, stdout.String())
	}
	if len(responses) != 2 ||
		string(responses[0].ID) != `"ab"` ||
		string(responses[1].ID) != "2" ||
		responses[0].Error.Code != daemonGenerationChangedErrorCode ||
		responses[1].Error.Code != daemonGenerationChangedErrorCode ||
		responses[0].Error.Data.Outcome != "unknown" ||
		responses[1].Error.Data.Outcome != "unknown" {
		t.Fatalf("batch unknown responses = %#v", responses)
	}
	if len(session.pending) != 0 {
		t.Fatalf("pending requests remain after failure: %#v", session.pending)
	}
}

func TestDaemonProxyLineReaderObservesEOFAfterQueuedFrames(t *testing.T) {
	var input strings.Builder
	for i := 0; i < daemonProxyEventBuffer+1; i++ {
		input.WriteString("queued\n")
	}
	events, done, stop := startDaemonProxyLineReader(newDaemonProxyScanner(strings.NewReader(input.String())))
	defer stop()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("queued host frames prevented EOF observation")
	}
	for range events {
	}
}

func TestDaemonProxySessionUsesSemanticStringIDsAndAllowsReuse(t *testing.T) {
	if left, right := daemonProxyIDKey(json.RawMessage(`"a\u0062"`)), daemonProxyIDKey(json.RawMessage(`"ab"`)); left == "" || left != right {
		t.Fatalf("semantic string ID keys differ: left=%q right=%q", left, right)
	}
	if left, right := daemonProxyIDKey(json.RawMessage(`1.0`)), daemonProxyIDKey(json.RawMessage(`1`)); left == "" || left != right {
		t.Fatalf("semantic numeric ID keys differ: left=%q right=%q", left, right)
	}

	session := newDaemonProxySession()
	session.observeHost([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n"))
	var stdout bytes.Buffer
	if err := session.forwardDaemon([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":true}}}}`+"\n"), &stdout); err != nil {
		t.Fatal(err)
	}
	session.observeHost([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"))
	if len(session.pending) != 1 {
		t.Fatalf("reused request ID was not tracked: %#v", session.pending)
	}
	if err := session.forwardDaemon([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`+"\n"), &stdout); err != nil {
		t.Fatal(err)
	}
	if len(session.pending) != 0 {
		t.Fatalf("reused request ID remained pending: %#v", session.pending)
	}
}

func TestDaemonProxySessionMatchesSDKNumericIDCoercion(t *testing.T) {
	session := newDaemonProxySession()
	session.observeHost([]byte(`{"jsonrpc":"2.0","id":9007199254740993,"method":"tools/list"}` + "\n"))
	if len(session.pending) != 1 {
		t.Fatalf("large numeric request ID was not tracked: %#v", session.pending)
	}

	var stdout bytes.Buffer
	if err := session.forwardDaemon([]byte(`{"jsonrpc":"2.0","id":9007199254740992,"result":{"tools":[]}}`+"\n"), &stdout); err != nil {
		t.Fatal(err)
	}
	if len(session.pending) != 0 {
		t.Fatalf("SDK-coerced numeric response ID remained pending: %#v", session.pending)
	}
}

func TestDaemonProxyResumeRejectsChangedHandshakeContract(t *testing.T) {
	session := establishedDaemonProxySession(t)
	connection := daemonProxyTestConnection(
		`{"jsonrpc":"2.0","id":"issueops-reconnect-1","result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{"listChanged":true}}}}` + "\n",
	)

	err := session.resume(context.Background(), connection, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "handshake contract changed") {
		t.Fatalf("changed handshake contract error = %v", err)
	}
}

func TestDaemonProxyHandshakeContractPreservesLargeCapabilityNumbers(t *testing.T) {
	var first daemonProxyEnvelope
	if err := json.Unmarshal([]byte(`{"id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"experimental":{"revision":9007199254740992}}}}`), &first); err != nil {
		t.Fatal(err)
	}
	var second daemonProxyEnvelope
	if err := json.Unmarshal([]byte(`{"id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"experimental":{"revision":9007199254740993}}}}`), &second); err != nil {
		t.Fatal(err)
	}
	session := newDaemonProxySession()
	if err := session.cacheInitializeContract(first); err != nil {
		t.Fatal(err)
	}
	if err := session.validateInitializeContract(second); !errors.Is(err, errDaemonProxyHandshakeChanged) {
		t.Fatalf("changed large-number capability error = %v", err)
	}
}

func TestDaemonProxyHandshakeRequiresCapabilitiesObject(t *testing.T) {
	for _, capabilities := range []string{"null", "[]", `"invalid"`, "1"} {
		t.Run(capabilities, func(t *testing.T) {
			var envelope daemonProxyEnvelope
			payload := `{"id":1,"result":{"protocolVersion":"2025-06-18","capabilities":` + capabilities + `}}`
			if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
				t.Fatal(err)
			}
			if _, _, err := daemonProxyInitializeContract(envelope); err == nil {
				t.Fatalf("capabilities %s must be rejected", capabilities)
			}
		})
	}
}

func TestDaemonProxyResumeDoesNotEstablishRejectedInitialize(t *testing.T) {
	session := newDaemonProxySession()
	session.observeHost([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n"))
	connection := daemonProxyTestConnection(
		`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"unsupported"}}` + "\n",
	)
	var stdout bytes.Buffer

	err := session.resume(context.Background(), connection, &stdout)
	if err == nil || !errors.Is(err, errDaemonProxyInitializeRejected) {
		t.Fatalf("rejected initialize error = %v", err)
	}
	if session.initializeResponseForwarded {
		t.Fatal("rejected initialize must not establish the proxy session")
	}
	if strings.Count(stdout.String(), `"unsupported"`) != 1 {
		t.Fatalf("initialize rejection must be forwarded exactly once: %q", stdout.String())
	}
}

func TestReconnectDaemonProxyTreatsMalformedHandshakeAsTerminal(t *testing.T) {
	dialCount := 0
	_, err := reconnectDaemonProxy(daemonProxyDeps{
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
		},
		dial: func(context.Context, string, string) (io.ReadWriteCloser, error) {
			dialCount++
			return &daemonProxyFakeConn{
				reader: strings.NewReader(`{"jsonrpc":"2.0","id":"issueops-reconnect-1","result":{"protocolVersion":"2025-06-18"}}` + "\n"),
			}, nil
		},
		reconnectTimeout: time.Second,
	}, establishedDaemonProxySession(t), make(chan struct{}))
	if !errors.Is(err, errDaemonProxyHandshakeChanged) || dialCount != 1 {
		t.Fatalf("malformed handshake = err %v dial count %d", err, dialCount)
	}
}

func TestReconnectDaemonProxyStopsWhenHostContextEnds(t *testing.T) {
	hostDone := make(chan struct{})
	close(hostDone)
	ensureCalls := 0
	_, err := reconnectDaemonProxy(daemonProxyDeps{
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			ensureCalls++
			return daemonStatus{}, errors.New("must not run")
		},
	}, newDaemonProxySession(), hostDone)
	if !errors.Is(err, io.EOF) || ensureCalls != 0 {
		t.Fatalf("host-closed reconnect = err %v ensure calls %d", err, ensureCalls)
	}
}

func TestReconnectDaemonProxyCancelsBlockedHandshakeOnHostEOF(t *testing.T) {
	proxy, daemon := net.Pipe()
	defer proxy.Close()
	defer daemon.Close()
	hostDone := make(chan struct{})
	ready := make(chan struct{})
	go func() {
		_, _ = bufio.NewReader(daemon).ReadString('\n')
		close(ready)
	}()
	done := make(chan error, 1)
	go func() {
		_, err := reconnectDaemonProxy(daemonProxyDeps{
			ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
				return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
			},
			dial: func(context.Context, string, string) (io.ReadWriteCloser, error) {
				return proxy, nil
			},
			reconnectTimeout: time.Second,
		}, establishedDaemonProxySession(t), hostDone)
		done <- err
	}()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("replayed initialize did not reach the blocked daemon")
	}
	close(hostDone)
	select {
	case err := <-done:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("host EOF reconnect error = %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("host EOF did not cancel the blocked replay handshake")
	}
}

func TestReconnectDaemonProxyUsesOneTotalDeadline(t *testing.T) {
	proxy, daemon := net.Pipe()
	defer proxy.Close()
	defer daemon.Close()
	hostDone := make(chan struct{})
	go func() {
		_, _ = bufio.NewReader(daemon).ReadString('\n')
	}()
	started := time.Now()
	_, err := reconnectDaemonProxy(daemonProxyDeps{
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
		},
		dial: func(context.Context, string, string) (io.ReadWriteCloser, error) {
			return proxy, nil
		},
		reconnectTimeout: 50 * time.Millisecond,
	}, establishedDaemonProxySession(t), hostDone)
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("total reconnect deadline error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 300*time.Millisecond {
		t.Fatalf("total reconnect deadline took %s", elapsed)
	}
}

func TestReconnectDaemonProxyRejectsHandshakeCompletedAfterHostEOF(t *testing.T) {
	hostDone := make(chan struct{})
	response := []byte(`{"jsonrpc":"2.0","id":"issueops-reconnect-1","result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":true}}}}` + "\n")
	reader := &daemonProxyCallbackReader{
		data: response,
		beforeRead: func() {
			close(hostDone)
		},
	}
	connection := &daemonProxyFakeConn{reader: reader}
	_, err := reconnectDaemonProxy(daemonProxyDeps{
		ensureDaemonRunning: func(context.Context) (daemonStatus, error) {
			return daemonStatus{Paths: daemonPaths{Socket: "daemon.sock"}}, nil
		},
		dial: func(context.Context, string, string) (io.ReadWriteCloser, error) {
			return connection, nil
		},
		stdout:           io.Discard,
		reconnectTimeout: time.Second,
	}, establishedDaemonProxySession(t), hostDone)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("handshake/EOF race error = %v", err)
	}
	if !connection.isClosed() {
		t.Fatal("handshake completed after host EOF returned an open connection")
	}
}

func establishedDaemonProxySession(t *testing.T) *daemonProxySession {
	t.Helper()
	session := newDaemonProxySession()
	session.observeHost([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n"))
	if err := session.forwardDaemon([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":true}}}}`+"\n"), io.Discard); err != nil {
		t.Fatal(err)
	}
	session.observeHost([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"))
	return session
}

func daemonProxyTestConnection(input string) *daemonProxyConnection {
	conn := &daemonProxyFakeConn{reader: strings.NewReader(input)}
	return &daemonProxyConnection{conn: conn, scanner: newDaemonProxyScanner(conn)}
}

type daemonProxyCallbackReader struct {
	data       []byte
	beforeRead func()
	read       bool
}

func (reader *daemonProxyCallbackReader) Read(buffer []byte) (int, error) {
	if reader.read {
		return 0, io.EOF
	}
	reader.read = true
	if reader.beforeRead != nil {
		reader.beforeRead()
	}
	return copy(buffer, reader.data), nil
}
