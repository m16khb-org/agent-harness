package mcpcli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleProjectMCPToolCallCoversLocalProjectPayloads(t *testing.T) {
	repo := t.TempDir()
	tests := []struct {
		name     string
		call     MCPToolCall
		wantText string
	}{
		{
			name:     "docs index",
			call:     MCPToolCall{Name: "docs_index", Arguments: map[string]any{}},
			wantText: "harness_root",
		},
		{
			name:     "skill manifest",
			call:     MCPToolCall{Name: "skill_manifest", Arguments: map[string]any{}},
			wantText: "skills",
		},
		{
			name:     "project route",
			call:     MCPToolCall{Name: "project_docs_route", Arguments: map[string]any{"repo": repo, "task": "api"}},
			wantText: "project_docs_route",
		},
		{
			name:     "project read missing",
			call:     MCPToolCall{Name: "project_docs_read", Arguments: map[string]any{"repo": repo, "rel_path": ".agent-harness/ARCHITECTURE.md"}},
			wantText: "document_missing",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleProjectMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err != nil || outcome.Direct {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
			text := mcpProjectPayloadText(t, outcome.Payload)
			if !strings.Contains(text, tc.wantText) {
				t.Fatalf("payload text = %s, want %q", text, tc.wantText)
			}
		})
	}
}

func TestHandleProjectMCPToolCallCoversProjectErrorBranches(t *testing.T) {
	repo := t.TempDir()
	tests := []struct {
		name        string
		call        MCPToolCall
		wantMessage string
		wantData    string
	}{
		{
			name: "project update missing content",
			call: MCPToolCall{Name: "project_docs_update", Arguments: map[string]any{
				"repo": repo, "rel_path": ".agent-harness/ARCHITECTURE.md", "summary": "update",
			}},
			wantMessage: "Project docs update failed",
			wantData:    "content is required",
		},
		{
			name: "project record unsupported kind",
			call: MCPToolCall{Name: "project_docs_record", Arguments: map[string]any{
				"repo": repo, "kind": "note", "title": "Title", "summary": "Summary",
			}},
			wantMessage: "Project docs record failed",
			wantData:    "unsupported record kind",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleProjectMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err == nil {
				t.Fatalf("expected handled MCP failure, got %#v", outcome)
			}
			if outcome.Err.Code != -32602 || outcome.Err.Message != tc.wantMessage || !strings.Contains(string(outcome.Err.Data), tc.wantData) {
				t.Fatalf("unexpected MCP error: %+v", outcome.Err)
			}
		})
	}
}

func TestHandleProjectMCPToolCallIgnoresUnknownProjectTool(t *testing.T) {
	outcome := handleProjectMCPToolCall(MCPToolCall{Name: "not_project_tool", Arguments: map[string]any{}})
	if outcome.Handled {
		t.Fatalf("unknown project tool should be ignored: %#v", outcome)
	}
}

func TestMCPResourceReadCoversGuidanceStateAndErrors(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, tc := range []struct {
		uri       string
		wantType  string
		wantText  string
		wantError bool
	}{
		{uri: "harness://project-doc-upkeep", wantType: "text/markdown", wantText: "Project Doc Upkeep Guidance"},
		{uri: "harness://api-doc-guidance", wantType: "text/markdown", wantText: "API Documentation Guidance"},
		{uri: "harness://command-policy", wantType: "application/json", wantText: "workspace"},
		{uri: "harness://state", wantType: "application/json", wantText: "keys"},
		{uri: "harness://commit-policy", wantType: "text/markdown", wantText: "Conventional"},
		{uri: "harness://unknown", wantError: true},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			result, rpcErr := HandleResourceRead(mustMarshalMCPTest(t, map[string]any{"uri": tc.uri}))
			if tc.wantError {
				if rpcErr == nil || !strings.Contains(rpcErr.Message, "Unknown resource") {
					t.Fatalf("expected unknown resource error, got result=%#v err=%+v", result, rpcErr)
				}
				return
			}
			if rpcErr != nil {
				t.Fatalf("HandleResourceRead(%s): %+v", tc.uri, rpcErr)
			}
			content := singleMCPResourceContent(t, result)
			if content["mimeType"] != tc.wantType || !strings.Contains(content["text"].(string), tc.wantText) {
				t.Fatalf("unexpected resource content for %s: %#v", tc.uri, content)
			}
		})
	}

	_, rpcErr := HandleResourceRead(json.RawMessage(`{bad json}`))
	if rpcErr == nil || rpcErr.Code != -32602 {
		t.Fatalf("expected invalid params error, got %+v", rpcErr)
	}
}

func TestMCPProjectToolCallCoversDirectPayloadAndUnknownTool(t *testing.T) {
	direct, rpcErr := HandleToolCall(mustMarshalMCPTest(t, map[string]any{"name": "commit_policy", "arguments": map[string]any{}}))
	if rpcErr != nil {
		t.Fatalf("commit_policy: %+v", rpcErr)
	}
	if text := extractSingleTextResult(t, direct); !strings.Contains(text, "Conventional") {
		t.Fatalf("commit_policy did not return markdown policy text: %s", text)
	}

	payload, rpcErr := HandleToolCall(mustMarshalMCPTest(t, map[string]any{"name": "project_docs_bootstrap_plan", "arguments": map[string]any{"repo": t.TempDir()}}))
	if rpcErr != nil {
		t.Fatalf("project_docs_bootstrap_plan: %+v", rpcErr)
	}
	if text := extractSingleTextResult(t, payload); !strings.Contains(text, `"dry_run"`) && !strings.Contains(text, `"write"`) {
		t.Fatalf("bootstrap plan payload did not look like JSON result: %s", text)
	}

	_, rpcErr = HandleToolCall(mustMarshalMCPTest(t, map[string]any{"name": "missing_tool", "arguments": map[string]any{}}))
	if rpcErr == nil || rpcErr.Code != -32602 || !strings.Contains(rpcErr.Message, "Unknown tool") {
		t.Fatalf("expected unknown tool error, got %+v", rpcErr)
	}
	_, rpcErr = HandleToolCall(json.RawMessage(`{bad json}`))
	if rpcErr == nil || rpcErr.Code != -32602 {
		t.Fatalf("expected invalid params error, got %+v", rpcErr)
	}
}

func mcpProjectPayloadText(t *testing.T, payload any) string {
	t.Helper()
	outcome := mcpToolPayload(payload)
	b, err := textFromMCPToolOutcome(outcome)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func textFromMCPToolOutcome(outcome MCPToolOutcome) (string, error) {
	b, err := jsonMarshalIndentForMCPProjectTest(outcome.Payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonMarshalIndentForMCPProjectTest(payload any) ([]byte, error) {
	return json.MarshalIndent(payload, "", "  ")
}

func singleMCPResourceContent(t *testing.T, result any) map[string]any {
	t.Helper()
	outer, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected resource result type %T", result)
	}
	contents, ok := outer["contents"].([]map[string]any)
	if !ok || len(contents) != 1 {
		t.Fatalf("unexpected resource contents: %#v", outer["contents"])
	}
	return contents[0]
}
