package mcpcli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops"
)

func TestServeMCPStreamContextCancelsIdleSDKSession(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeMCPStreamContext(ctx, server, server, io.Discard)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SDK MCP session ignored cancellation")
	}
}

func TestServeMCPStreamListsHarnessTools(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	var diag bytes.Buffer
	err := ServeMCPStream(strings.NewReader(input), &out, &diag)
	// SDK returns error when input ends; responses are already written.
	if err != nil && !strings.Contains(err.Error(), "server is closing") && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("ServeMCPStream: %v\ndiag:\n%s", err, diag.String())
	}
	output := out.String()
	if output == "" {
		t.Fatal("no output from ServeMCPStream")
	}
	lines := splitLines(output)
	// There should be at least 3 responses (initialize, tools/list, resources/read)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 responses, got %d: %s", len(lines), output)
	}
	var hasInit, hasTools, hasResource bool
	for _, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid json %q: %v", line, err)
		}
		if _, ok := obj["result"]; !ok {
			continue
		}
		result := obj["result"].(map[string]any)
		if _, ok := result["serverInfo"]; ok {
			hasInit = true
		}
		if tools, ok := result["tools"]; ok {
			if toolsArr, ok := tools.([]any); ok && len(toolsArr) > 0 {
				hasTools = true
			}
		}
		if _, ok := result["contents"]; ok {
			hasResource = true
		}
	}
	if !hasInit || !hasTools || !hasResource {
		t.Fatalf("missing responses (init=%v tools=%v resource=%v):\n%s", hasInit, hasTools, hasResource, output)
	}
}

func TestServeMCPStreamCarriesPublicationReconcileWithoutInvokingOnHandshake(t *testing.T) {
	invoked := 0
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	err := ServeMCPStreamWithDependencies(strings.NewReader(input), &output, io.Discard, MCPDependencies{
		Publication: issueops.RemotePublicationHandlers{Reconcile: func(context.Context, string, issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
			invoked++
			return issueops.ExecutionReconcileResult{}, nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invoked != 0 {
		t.Fatalf("stream handshake invoked publication reconcile handler: %d", invoked)
	}
	if !strings.Contains(output.String(), `"serverInfo"`) || !strings.Contains(output.String(), `"tools"`) {
		t.Fatalf("stream did not complete handshake and tool listing: %s", output.String())
	}
}
