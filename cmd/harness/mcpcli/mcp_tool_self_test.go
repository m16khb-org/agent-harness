package mcpcli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleSelfLoopMCPToolCallCoversLocalPayloads(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	tests := []struct {
		name     string
		call     MCPToolCall
		wantText string
	}{
		{
			name: "self augment plan",
			call: MCPToolCall{Name: "self_augment", Arguments: map[string]any{
				"cycles": 2,
			}},
			wantText: `"self_augmentation"`,
		},
		{
			name: "self augment plan save state",
			call: MCPToolCall{Name: "self_augment", Arguments: map[string]any{
				"save_state": true,
				"state_key":  "mcp-self-augment-plan",
			}},
			wantText: `"state_checkpoint"`,
		},
		{
			name: "self augment lesson",
			call: MCPToolCall{Name: "self_augment_lesson", Arguments: map[string]any{
				"candidate_id": "candidate-refill-curriculum",
				"lesson":       "Keep MCP self dispatch local in tests.",
				"next_action":  "Pin the next safe branch.",
			}},
			wantText: `"self_augmentation_lesson"`,
		},
		{
			name:     "self verify candidates",
			call:     MCPToolCall{Name: "self_verify_candidates", Arguments: map[string]any{}},
			wantText: `"self_verification_candidate_export"`,
		},
		{
			name:     "self verify candidates save state",
			call:     MCPToolCall{Name: "self_verify_candidates", Arguments: map[string]any{"save_state": true, "state_key": "mcp-self-candidates"}},
			wantText: `"state_checkpoint"`,
		},
		{
			name:     "self verify history",
			call:     MCPToolCall{Name: "self_verify_history", Arguments: map[string]any{"prefix": "mcp-self", "limit": 5}},
			wantText: `"entries"`,
		},
		{
			name:     "self augment history alias",
			call:     MCPToolCall{Name: "self_augment_history", Arguments: map[string]any{"prefix": "mcp-self", "limit": 5}},
			wantText: `"entries"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleSelfLoopMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err != nil {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
			if text := mcpSelfPayloadText(t, outcome.Payload); !strings.Contains(text, tc.wantText) {
				t.Fatalf("payload text = %s, want %q", text, tc.wantText)
			}
		})
	}
}

func TestHandleSelfLoopMCPToolCallCoversBoundaryErrorsAndUnknownTool(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	tests := []struct {
		name    string
		call    MCPToolCall
		wantMsg string
	}{
		{
			name:    "self augment lesson missing lesson",
			call:    MCPToolCall{Name: "self_augment_lesson", Arguments: map[string]any{"candidate_id": "candidate-refill-curriculum", "next_action": "y"}},
			wantMsg: "Self-augmentation lesson save failed",
		},
		{
			name:    "self verify invalid mode",
			call:    MCPToolCall{Name: "self_verify", Arguments: map[string]any{"full": true, "iterations": 1}},
			wantMsg: "Self-verification mode invalid",
		},
		{
			name:    "self verify compare missing baseline",
			call:    MCPToolCall{Name: "self_verify_compare", Arguments: map[string]any{"candidate_key": "candidate"}},
			wantMsg: "Self-verify compare failed",
		},
		{
			name:    "self augment compare alias missing baseline",
			call:    MCPToolCall{Name: "self_augment_compare", Arguments: map[string]any{"candidate_key": "candidate"}},
			wantMsg: "Self-verify compare failed",
		},
		{
			name:    "self verify promote missing source",
			call:    MCPToolCall{Name: "self_verify_promote", Arguments: map[string]any{"baseline_key": "baseline", "confirm": true}},
			wantMsg: "Self-verify promote failed",
		},
		{
			name:    "self augment promote alias missing source",
			call:    MCPToolCall{Name: "self_augment_promote", Arguments: map[string]any{"baseline_key": "baseline", "confirm": true}},
			wantMsg: "Self-verify promote failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleSelfLoopMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err == nil || outcome.Err.Message != tc.wantMsg {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
		})
	}

	unknown := handleSelfLoopMCPToolCall(MCPToolCall{Name: "not_self_loop", Arguments: map[string]any{}})
	if unknown.Handled {
		t.Fatalf("unknown self-loop tool should pass through: %#v", unknown)
	}
}

func mcpSelfPayloadText(t *testing.T, payload any) string {
	t.Helper()
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
