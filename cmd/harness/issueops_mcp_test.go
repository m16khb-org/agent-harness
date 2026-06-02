package main

import (
	"encoding/json"
	"testing"
)

func TestMCPIssueOpsStartAndStatus(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "feature/demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" || start["phase"] != "problem" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	status := callMCPToolForIssueOpsTest(t, "issueops_status", map[string]any{"id": id})
	if status["id"] != id || status["repo"] != "/repo/example" {
		t.Fatalf("unexpected MCP status payload: %#v", status)
	}
}

func callMCPToolForIssueOpsTest(t *testing.T, name string, args map[string]any) map[string]any {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := handleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("unexpected MCP rpc error: %+v", rpcErr)
	}
	outer, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected MCP result type %T", result)
	}
	content, ok := outer["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatalf("unexpected MCP content: %#v", outer["content"])
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("unexpected MCP text content: %#v", content[0])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("invalid MCP JSON text: %v\n%s", err, text)
	}
	return payload
}
