package mcpcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlePolicyStateMCPToolCallCoversPolicyPayloads(t *testing.T) {
	repo := t.TempDir()
	tests := []struct {
		name     string
		call     MCPToolCall
		wantText string
	}{
		{
			name: "command policy check",
			call: MCPToolCall{Name: "command_policy_check", Arguments: map[string]any{
				"workspace_root": repo,
				"cwd":            repo,
				"argv":           []any{"git", "status", "--short"},
			}},
			wantText: "allowed",
		},
		{
			name: "command fake run",
			call: MCPToolCall{Name: "command_fake_run", Arguments: map[string]any{
				"workspace_root": repo,
				"cwd":            repo,
				"argv":           []any{"git", "status", "--short"},
			}},
			wantText: "fake",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handlePolicyStateMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err != nil || outcome.Direct {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
			text := mcpPolicyStatePayloadText(t, outcome.Payload)
			if !strings.Contains(text, tc.wantText) {
				t.Fatalf("payload text = %s, want %q", text, tc.wantText)
			}
		})
	}
}

func TestHandlePolicyStateMCPToolCallUsesWorkspacePolicyOverride(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "policy.json"), []byte(`{"additional_read_only_commands":["repo-tool"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := handlePolicyStateMCPToolCall(MCPToolCall{Name: "command_policy_check", Arguments: map[string]any{
		"workspace_root": repo,
		"cwd":            repo,
		"argv":           []any{"repo-tool"},
	}})
	if !outcome.Handled || outcome.Err != nil || outcome.Direct {
		t.Fatalf("unexpected MCP outcome: %#v", outcome)
	}
	text := mcpPolicyStatePayloadText(t, outcome.Payload)
	if !strings.Contains(text, `"allowed": true`) {
		t.Fatalf("workspace policy override was not reflected in MCP payload: %s", text)
	}
}

func TestHandlePolicyStateMCPToolCallCoversStatePayloads(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	write := handlePolicyStateMCPToolCall(MCPToolCall{Name: "state_write", Arguments: map[string]any{
		"key": "mcp-policy-state-test", "content": "payload",
	}})
	if !write.Handled || write.Err != nil || !strings.Contains(mcpPolicyStatePayloadText(t, write.Payload), "mcp-policy-state-test") {
		t.Fatalf("unexpected state_write outcome: %#v", write)
	}

	for _, tc := range []struct {
		name     string
		call     MCPToolCall
		wantText string
	}{
		{name: "state read", call: MCPToolCall{Name: "state_read", Arguments: map[string]any{"key": "mcp-policy-state-test"}}, wantText: "payload"},
		{name: "state list", call: MCPToolCall{Name: "state_list", Arguments: map[string]any{}}, wantText: "mcp-policy-state-test"},
		{name: "state prune dry run", call: MCPToolCall{Name: "state_prune", Arguments: map[string]any{"max_age": "1h"}}, wantText: "dry_run"},
		{name: "state doctor", call: MCPToolCall{Name: "state_doctor", Arguments: map[string]any{}}, wantText: "healthy"},
		{name: "state migrate dry run", call: MCPToolCall{Name: "state_migrate", Arguments: map[string]any{}}, wantText: "dry_run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handlePolicyStateMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err != nil {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
			if text := mcpPolicyStatePayloadText(t, outcome.Payload); !strings.Contains(text, tc.wantText) {
				t.Fatalf("payload text = %s, want %q", text, tc.wantText)
			}
		})
	}
}

func TestHandlePolicyStateMCPToolCallCoversErrorsAndUnknownTool(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	prune := handlePolicyStateMCPToolCall(MCPToolCall{Name: "state_prune", Arguments: map[string]any{"max_age": "not-a-duration"}})
	if !prune.Handled || prune.Err == nil || prune.Err.Code != -32602 || prune.Err.Message != "State prune failed" || !strings.Contains(prune.Err.Data.(string), "invalid max_age") {
		t.Fatalf("unexpected state_prune error outcome: %#v", prune)
	}

	read := handlePolicyStateMCPToolCall(MCPToolCall{Name: "state_read", Arguments: map[string]any{"key": ""}})
	if !read.Handled || read.Err == nil || read.Err.Code != -32602 || read.Err.Message != "State read failed" {
		t.Fatalf("unexpected state_read error outcome: %#v", read)
	}

	unknown := handlePolicyStateMCPToolCall(MCPToolCall{Name: "not_policy_state", Arguments: map[string]any{}})
	if unknown.Handled {
		t.Fatalf("unknown policy/state tool should pass through: %#v", unknown)
	}
}

func mcpPolicyStatePayloadText(t *testing.T, payload any) string {
	t.Helper()
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
