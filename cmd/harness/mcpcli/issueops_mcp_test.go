package mcpcli

import (
	"fmt"
	"strings"
	"testing"
)

func TestMCPIssueOpsStartAndStatus(t *testing.T) {
	configureIssueOpsMCPForTest(t)
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

func TestMCPIssueOpsSetPhasePinsIntentAndIssuePlanGate(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intentErr := callMCPToolForIssueOpsTestError(t, "issueops_set_phase", map[string]any{"id": id, "to": "plan"})
	if intentErr == nil || !strings.Contains(fmt.Sprint(intentErr.Data), "intent_contract") {
		t.Fatalf("expected MCP plan phase to require intent, got %+v", intentErr)
	}
	callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "IssueOps must understand intent",
		"interpreted_intent": "Persist main-agent intent before planning",
		"success_criteria":   []string{"intent is recorded"},
	})
	issueErr := callMCPToolForIssueOpsTestError(t, "issueops_set_phase", map[string]any{"id": id, "to": "plan"})
	if issueErr == nil || !strings.Contains(fmt.Sprint(issueErr.Data), "issue_url") {
		t.Fatalf("expected MCP plan phase to require linked issue after intent, got %+v", issueErr)
	}
}

func TestMCPIssueOpsLinkChild(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	stubIssueOpsChildIssueVerifierForMCPTest(t, nil)
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
	configureIssueOpsMCPForTest(t)
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

func TestMCPIssueOpsMarkIssueUpdated(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_add_feedback", map[string]any{
		"id":             id,
		"source":         "review",
		"body":           "acceptance criteria changed",
		"classification": "contract_change",
	})
	record := callMCPToolForIssueOpsTest(t, "issueops_mark_issue_updated", map[string]any{"id": id})
	feedback, ok := record["feedback"].([]any)
	if !ok || len(feedback) != 1 {
		t.Fatalf("expected one feedback item, got %#v", record)
	}
	item, ok := feedback[0].(map[string]any)
	if !ok || item["issue_updated_at"] == "" {
		t.Fatalf("expected issue_updated_at after MCP mark, got %#v", feedback[0])
	}
}
