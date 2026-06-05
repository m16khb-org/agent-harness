package main

import (
	"encoding/json"
	"testing"
)

func TestMCPIssueOpsStartAndStatus(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" || start["phase"] != "problem" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	status := callMCPToolForIssueOpsTest(t, "issueops_status", map[string]any{"id": id})
	if status["id"] != id || status["repo"] != "/repo/example" {
		t.Fatalf("unexpected MCP status payload: %#v", status)
	}
}

func TestMCPIssueOpsLinkChild(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://gitlab.example/group/project/-/issues/1",
	})
	record := callMCPToolForIssueOpsTest(t, "issueops_link_child", map[string]any{
		"id":        id,
		"child_url": "https://gitlab.example/group/project/-/issues/2",
		"title":     "write GitLab child task",
	})
	links, ok := record["issue_links"].([]any)
	if !ok || len(links) != 1 {
		t.Fatalf("expected one issue link, got %#v", record)
	}
	link, ok := links[0].(map[string]any)
	if !ok || link["type"] != "child" || link["provider"] != "gitlab" {
		t.Fatalf("unexpected issue link: %#v", links[0])
	}
}

func TestMCPIssueOpsPrepareBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "123-provider-linked-branch"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://gitlab.example/group/project/-/issues/123",
	})
	record := callMCPToolForIssueOpsTest(t, "issueops_prepare_branch", map[string]any{
		"id":          id,
		"provider":    "gitlab",
		"issue_url":   "https://gitlab.example/group/project/-/issues/123",
		"branch":      "123-provider-linked-branch",
		"base_branch": "main",
	})
	prepare, ok := record["branch_prepare"].(map[string]any)
	if !ok || prepare["provider"] != "gitlab" || prepare["branch"] != "123-provider-linked-branch" {
		t.Fatalf("unexpected branch prepare payload: %#v", record)
	}
	steps, ok := prepare["steps"].([]any)
	if !ok || len(steps) != 3 {
		t.Fatalf("expected provider fallback steps: %#v", prepare)
	}
	first, ok := steps[0].(map[string]any)
	if !ok || first["strategy"] != "mcp" || first["tool"] != "mcp__glab.glab_api" {
		t.Fatalf("first branch prepare step must use GitLab MCP: %#v", steps[0])
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
