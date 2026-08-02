package mcpcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"

	"agent-harness/internal/core/issueops"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRunMCPDirectUsesStreamTransport(t *testing.T) {
	t.Setenv("HARNESS_MCP_DIRECT", "1")
	session := startRunMCPTestSession(t, MCPDependencies{})
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) == 0 {
		t.Fatalf("RunMCP direct tool listing failed: tools=%#v err=%v", tools, err)
	}
}

func TestRunMCPWithDependenciesUsesItsReleaseHandlerOnDirectTransport(t *testing.T) {
	t.Setenv("HARNESS_MCP_DIRECT", "1")
	called := false
	session := startRunMCPTestSession(t, MCPDependencies{Release: func(_ context.Context, _ string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
		called = true
		return issueops.ExecutionResult{OK: true, ID: request.ID}, nil
	}})
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "issueops_execution", Arguments: map[string]any{"action": "release", "id": "io-mcp-direct", "generation": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("direct MCP transport did not invoke its injected release handler")
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, `"id": "io-mcp-direct"`) {
		t.Fatalf("direct MCP response did not use injected handler result: %#v", result)
	}
}

func TestServeMCPStreamWithDependenciesKeepsConcurrentReleaseHandlersIsolated(t *testing.T) {
	type testCase struct {
		id string
	}
	cases := []testCase{{id: "io-mcp-first"}, {id: "io-mcp-second"}}
	called := make(chan string, len(cases))
	errs := make(chan error, len(cases))
	var group sync.WaitGroup
	for _, tc := range cases {
		group.Add(1)
		go func(tc testCase) {
			defer group.Done()
			server := initSDKServer(MCPDependencies{Release: func(_ context.Context, _ string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
				called <- tc.id
				return issueops.ExecutionResult{OK: true, ID: request.ID}, nil
			}})
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(context.Background(), serverTransport, nil)
			if err != nil {
				errs <- err
				return
			}
			defer serverSession.Close()
			client := mcp.NewClient(&mcp.Implementation{Name: "transport-test", Version: "1"}, nil)
			clientSession, err := client.Connect(context.Background(), clientTransport, nil)
			if err != nil {
				errs <- err
				return
			}
			defer clientSession.Close()
			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "issueops_execution", Arguments: map[string]any{"action": "release", "id": tc.id, "generation": 1}})
			if err != nil || len(result.Content) != 1 || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, tc.id) {
				errs <- fmt.Errorf("response did not contain %s", tc.id)
			}
		}(tc)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	close(called)
	seen := map[string]bool{}
	for id := range called {
		seen[id] = true
	}
	for _, tc := range cases {
		if !seen[tc.id] {
			t.Fatalf("release handler for %s was not invoked", tc.id)
		}
	}
}

func TestSDKTransportOwnsBothStdioAndDaemonConnections(t *testing.T) {
	source, err := os.ReadFile("mcp_transport.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"serveMCPStream" + "Legacy", "type RPC" + "Request", "type RPC" + "Error", "canUseSDK" + "Transport"} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("SDK transport does not exclusively own MCP streams: found %q", forbidden)
		}
	}

	for _, mode := range []string{"stdio", "daemon_conn"} {
		t.Run(mode, func(t *testing.T) {
			session := startMCPTransportTestSession(t, mode, MCPDependencies{})
			tools, err := session.ListTools(context.Background(), nil)
			if err != nil || !containsSDKTool(tools.Tools, "issueops_execution") {
				t.Fatalf("SDK tool listing failed: tools=%#v err=%v", tools, err)
			}
			resource, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "harness://commit-policy"})
			if err != nil || len(resource.Contents) == 0 {
				t.Fatalf("SDK resource read failed: resource=%#v err=%v", resource, err)
			}
		})
	}
}

func TestSDKTransportPreservesStructuredToolErrors(t *testing.T) {
	session := startMCPTransportTestSession(t, "stdio", MCPDependencies{})
	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "state_prune", Arguments: map[string]any{"max_age": "not-a-duration"},
	})
	if err == nil {
		t.Fatal("invalid tool arguments unexpectedly succeeded")
	}
	var protocolErr *jsonrpc.Error
	if !errors.As(err, &protocolErr) {
		t.Fatalf("tool failure was not a structured JSON-RPC error: %T %v", err, err)
	}
	if protocolErr.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("protocol error code = %d, want %d", protocolErr.Code, jsonrpc.CodeInvalidParams)
	}
}

func containsSDKTool(tools []*mcp.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func startMCPTransportTestSession(t *testing.T, mode string, deps MCPDependencies) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var reader io.ReadCloser
	var writer io.WriteCloser
	var closeClient func()
	switch mode {
	case "stdio":
		serverReader, clientWriter := io.Pipe()
		clientReader, serverWriter := io.Pipe()
		go func() {
			done <- ServeMCPStreamContextWithDependencies(ctx, serverReader, serverWriter, io.Discard, deps)
		}()
		reader, writer = clientReader, clientWriter
		closeClient = func() { _ = clientReader.Close(); _ = clientWriter.Close() }
	case "daemon_conn":
		serverConn, clientConn := net.Pipe()
		go func() { done <- ServeMCPStreamContextWithDependencies(ctx, serverConn, serverConn, io.Discard, deps) }()
		reader, writer = clientConn, clientConn
		closeClient = func() { _ = clientConn.Close() }
	default:
		t.Fatalf("unknown transport mode %q", mode)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "transport-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: reader, Writer: writer}, nil)
	if err != nil {
		cancel()
		closeClient()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		closeClient()
		<-done
	})
	return session
}

func startRunMCPTestSession(t *testing.T, deps MCPDependencies) *mcp.ClientSession {
	t.Helper()
	oldStdin, oldStdout, oldStderr := os.Stdin, os.Stdout, os.Stderr
	serverReader, clientWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	clientReader, serverWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin, os.Stdout, os.Stderr = serverReader, serverWriter, diagnostics
	done := make(chan error, 1)
	go func() { done <- RunMCPWithDependencies(deps) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "direct-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.IOTransport{Reader: clientReader, Writer: clientWriter}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		_ = clientWriter.Close()
		_ = clientReader.Close()
		_ = serverReader.Close()
		_ = serverWriter.Close()
		_ = diagnostics.Close()
		os.Stdin, os.Stdout, os.Stderr = oldStdin, oldStdout, oldStderr
		<-done
	})
	return session
}
