package hookcli

import (
	"strings"
	"testing"
)

func TestRunHookPreToolUseEnforcesSearchRoutingForStructuralSourceSearch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg -n \"func Run\" cmd internal"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "block" || obj["tool"] != "Bash" {
		t.Fatalf("expected enforced source search to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "CodeGraph") || !strings.Contains(reason, "codegraph_context") {
		t.Fatalf("block reason should point agents to CodeGraph, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsLiteralEvidenceSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg \"response_contracts\" cmd/harness/testdata README.md"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected docs/golden literal search to remain allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseHostJSONBlocksWhenSearchRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"grep -R \"type Hook\" internal/core"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected host hook JSON to block enforced source search, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsExternalSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"grep -R \"PostToolUse\" -n /Applications/Codex.app/Contents/Resources"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected external binary/app search to remain allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseClaudeHostUsesPermissionDecisionWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg \"type Hook\" internal/core"}}`, func() error {
		return runHookPreToolUse([]string{"--host", "claude", "--enforce-search-routing"})
	})
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PreToolUse" || hso["permissionDecision"] != "deny" {
		t.Fatalf("expected Claude PreToolUse permission denial, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksCodeGraphForExactSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"codegraph_context","tool_input":{"query":"DATABASE_URL"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected exact CodeGraph query to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "Use rg first") {
		t.Fatalf("block reason should point agents to rg, got %q", reason)
	}
}
