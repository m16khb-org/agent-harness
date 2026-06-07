package mcpcli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunMCPDirectUsesStreamTransport(t *testing.T) {
	t.Setenv("HARNESS_MCP_DIRECT", "1")
	stdout, stderr, err := captureMCPStdio(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n", RunMCP)
	if err != nil {
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

	stderr, err := captureProjectCLIStderr(func() error {
		handleNotification(RPCRequest{Method: "notifications/initialized"})
		return nil
	})
	if err != nil {
		t.Fatalf("notification wrapper failed: %v", err)
	}
	if !strings.Contains(stderr, "agent-harness mcp notification: notifications/initialized") {
		t.Fatalf("unexpected notification stderr:\n%s", stderr)
	}
}

func TestMCPTransportCoversParseNotificationAndMethodErrors(t *testing.T) {
	var out bytes.Buffer
	var diagnostics bytes.Buffer
	input := strings.NewReader(strings.Join([]string{
		"{not json}",
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":1,"method":"missing/method","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`,
		"",
	}, "\n"))

	if err := ServeMCPStream(input, &out, &diagnostics); err != nil {
		t.Fatalf("ServeMCPStream: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, `"Parse error"`) || !strings.Contains(output, `"Method not found"`) || !strings.Contains(output, `"serverInfo"`) {
		t.Fatalf("unexpected MCP output:\n%s", output)
	}
	if !strings.Contains(diagnostics.String(), "notifications/initialized") {
		t.Fatalf("notification was not written to diagnostics: %s", diagnostics.String())
	}

	out.Reset()
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
	outBytes, readOutErr := io.ReadAll(outR)
	errBytes, readErrErr := io.ReadAll(errR)
	_ = inR.Close()
	_ = outR.Close()
	_ = errR.Close()
	if closeOutErr != nil {
		t.Fatal(closeOutErr)
	}
	if closeErrErr != nil {
		t.Fatal(closeErrErr)
	}
	if readOutErr != nil {
		t.Fatal(readOutErr)
	}
	if readErrErr != nil {
		t.Fatal(readErrErr)
	}
	return string(outBytes), string(errBytes), callErr
}
