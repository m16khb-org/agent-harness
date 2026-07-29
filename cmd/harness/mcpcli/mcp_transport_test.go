package mcpcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestRunMCPDirectUsesStreamTransport(t *testing.T) {
	t.Setenv("HARNESS_MCP_DIRECT", "1")
	stdout, stderr, err := captureMCPStdio(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n"+
			`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n",
		RunMCP)
	// SDK returns EOF error when input closes; the response was already written.
	if err != nil && !strings.Contains(err.Error(), "server is closing") && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("RunMCP direct failed: %v\nstderr:\n%s", err, stderr)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &response); err != nil {
		t.Fatalf("decode RunMCP response: %v\n%s", err, stdout)
	}
	if response["id"].(float64) != 1 {
		t.Fatalf("unexpected response id: %#v", response)
	}
	result := response["result"].(map[string]any)
	serverInfo := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "agent_harness" {
		t.Fatalf("unexpected server info: %#v", serverInfo)
	}
}

func TestRunMCPWithDependenciesUsesItsReleaseHandlerOnDirectTransport(t *testing.T) {
	t.Setenv("HARNESS_MCP_DIRECT", "1")
	called := false
	stdout, stderr, err := captureMCPStdio(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"issueops_execution","arguments":{"action":"release","id":"io-mcp-direct","generation":1}}}`+"\n",
		func() error {
			return RunMCPWithDependencies(MCPDependencies{Release: func(_ context.Context, _ string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
				called = true
				return issueops.ExecutionResult{OK: true, ID: request.ID}, nil
			}})
		},
	)
	if err != nil && !strings.Contains(err.Error(), "server is closing") && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("RunMCPWithDependencies direct failed: %v\nstderr:\n%s", err, stderr)
	}
	if !called {
		t.Fatal("direct MCP transport did not invoke its injected release handler")
	}
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode direct MCP response: %v\n%s", err, stdout)
	}
	if len(response.Result.Content) != 1 || !strings.Contains(response.Result.Content[0].Text, `"id": "io-mcp-direct"`) {
		t.Fatalf("direct MCP response did not use injected handler result:\n%s", stdout)
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
			input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"issueops_execution","arguments":{"action":"release","id":"` + tc.id + `","generation":1}}}` + "\n"
			var output bytes.Buffer
			if err := ServeMCPStreamWithDependencies(strings.NewReader(input), &output, io.Discard, MCPDependencies{Release: func(_ context.Context, _ string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
				called <- tc.id
				return issueops.ExecutionResult{OK: true, ID: request.ID}, nil
			}}); err != nil {
				errs <- err
				return
			}
			if !strings.Contains(output.String(), tc.id) {
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

func TestMCPTransportStdoutAndStderrWrappers(t *testing.T) {
	resultOut := captureStatusVerifyStdout(t, func() error {
		writeRPCResult(json.RawMessage(`"abc"`), map[string]any{"ok": true})
		return nil
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(resultOut), &result); err != nil {
		t.Fatalf("decode result wrapper output: %v\n%s", err, resultOut)
	}
	if result["id"] != "abc" || result["jsonrpc"] != "2.0" {
		t.Fatalf("unexpected result wrapper payload: %#v", result)
	}

	errorOut := captureStatusVerifyStdout(t, func() error {
		writeRPCError(json.RawMessage(`7`), -32601, "Method not found", "missing")
		return nil
	})
	var errorPayload map[string]any
	if err := json.Unmarshal([]byte(errorOut), &errorPayload); err != nil {
		t.Fatalf("decode error wrapper output: %v\n%s", err, errorOut)
	}
	if errorPayload["id"].(float64) != 7 {
		t.Fatalf("unexpected error wrapper payload: %#v", errorPayload)
	}

	stderr, err := captureProjectCLIStderr(t, func() error {
		handleNotification(RPCRequest{Method: "notifications/initialized"})
		return nil
	})
	if err != nil {
		t.Fatalf("notification wrapper failed: %v", err)
	}
	if strings.Contains(stderr, "notifications/initialized") {
		t.Fatalf("notifications/initialized should be suppressed from diagnostics:\n%s", stderr)
	}

	stderr2, err2 := captureProjectCLIStderr(t, func() error {
		handleNotification(RPCRequest{Method: "notifications/something-else"})
		return nil
	})
	if err2 != nil {
		t.Fatalf("notification wrapper failed: %v", err2)
	}
	if !strings.Contains(stderr2, "agent-harness mcp notification: notifications/something-else") {
		t.Fatalf("non-initialized notification should be logged:\n%s", stderr2)
	}
}

// Note: this exercises the LEGACY transport (M1): strings.NewReader is not an
// io.ReadWriter, so canUseSDKTransport returns false and ServeMCPStream routes
// to serveMCPStreamLegacy. serveMCPStreamLegacy is load-bearing for the split
// reader/writer stdio path (HARNESS_MCP_DIRECT MCP smoke), so both transports
// are intentionally kept.
func TestMCPTransportCoversInitAndToolsLegacy(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	var diag bytes.Buffer
	err := ServeMCPStream(strings.NewReader(input), &out, &diag)
	// SDK returns an error when the input stream ends; responses are already written.
	if err != nil && !strings.Contains(err.Error(), "server is closing") && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("ServeMCPStream: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, `"serverInfo"`) || !strings.Contains(output, `"name":"agent_harness"`) {
		t.Fatalf("missing server info in output:\n%s", output)
	}
	if !strings.Contains(output, "atomic_commit_preflight") {
		t.Fatalf("missing tools in output:\n%s", output)
	}
	if !strings.Contains(output, `harness://commit-policy`) {
		t.Fatalf("missing resource read response in output:\n%s", output)
	}
}

func TestMCPTransportErrorResponseFormat(t *testing.T) {
	var out bytes.Buffer
	writeRPCErrorTo(&out, nil, -32000, "boom", "data")
	if !strings.Contains(out.String(), `"id":null`) || !strings.Contains(out.String(), `"boom"`) {
		t.Fatalf("writeRPCErrorTo did not preserve null id error response: %s", out.String())
	}
}

func captureMCPStdio(t *testing.T, stdin string, fn func() error) (string, string, error) {
	t.Helper()
	oldStdin, oldStdout, oldStderr := os.Stdin, os.Stdout, os.Stderr
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	type readResult struct {
		out []byte
		err error
	}
	outDone := make(chan readResult, 1)
	errDone := make(chan readResult, 1)
	go func() {
		out, err := io.ReadAll(outR)
		outDone <- readResult{out: out, err: err}
	}()
	go func() {
		out, err := io.ReadAll(errR)
		errDone <- readResult{out: out, err: err}
	}()
	if _, err := inW.WriteString(stdin); err != nil {
		t.Fatal(err)
	}
	if err := inW.Close(); err != nil {
		t.Fatal(err)
	}

	os.Stdin, os.Stdout, os.Stderr = inR, outW, errW
	callErr := fn()
	closeOutErr := outW.Close()
	closeErrErr := errW.Close()
	outRead := <-outDone
	errRead := <-errDone
	_ = inR.Close()
	_ = outR.Close()
	_ = errR.Close()
	if closeOutErr != nil {
		t.Fatal(closeOutErr)
	}
	if closeErrErr != nil {
		t.Fatal(closeErrErr)
	}
	if outRead.err != nil {
		t.Fatal(outRead.err)
	}
	if errRead.err != nil {
		t.Fatal(errRead.err)
	}
	return string(outRead.out), string(errRead.out), callErr
}
