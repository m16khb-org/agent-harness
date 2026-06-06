package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleProjectMCPToolCallCoversLocalProjectPayloads(t *testing.T) {
	repo := t.TempDir()
	tests := []struct {
		name     string
		call     mcpToolCall
		wantText string
	}{
		{
			name:     "docs index",
			call:     mcpToolCall{Name: "docs_index", Arguments: map[string]any{}},
			wantText: "harness_root",
		},
		{
			name:     "skill manifest",
			call:     mcpToolCall{Name: "skill_manifest", Arguments: map[string]any{}},
			wantText: "skills",
		},
		{
			name:     "project route",
			call:     mcpToolCall{Name: "project_docs_route", Arguments: map[string]any{"repo": repo, "task": "api"}},
			wantText: "project_docs_route",
		},
		{
			name:     "project read missing",
			call:     mcpToolCall{Name: "project_docs_read", Arguments: map[string]any{"repo": repo, "rel_path": ".agent-harness/ARCHITECTURE.md"}},
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
		call        mcpToolCall
		wantMessage string
		wantData    string
	}{
		{
			name: "project update missing content",
			call: mcpToolCall{Name: "project_docs_update", Arguments: map[string]any{
				"repo": repo, "rel_path": ".agent-harness/ARCHITECTURE.md", "summary": "update",
			}},
			wantMessage: "Project docs update failed",
			wantData:    "content is required",
		},
		{
			name: "project record unsupported kind",
			call: mcpToolCall{Name: "project_docs_record", Arguments: map[string]any{
				"repo": repo, "kind": "note", "title": "Title", "summary": "Summary",
			}},
			wantMessage: "Project docs record failed",
			wantData:    "unsupported record kind",
		},
		{
			name:        "api doc review invalid timeout",
			call:        mcpToolCall{Name: "api_doc_review", Arguments: map[string]any{"repo": repo, "timeout": "not-a-duration"}},
			wantMessage: "API doc review failed",
			wantData:    "invalid timeout",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleProjectMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err == nil {
				t.Fatalf("expected handled MCP failure, got %#v", outcome)
			}
			if outcome.Err.Code != -32602 || outcome.Err.Message != tc.wantMessage || !strings.Contains(outcome.Err.Data.(string), tc.wantData) {
				t.Fatalf("unexpected MCP error: %+v", outcome.Err)
			}
		})
	}
}

func TestHandleProjectMCPToolCallIgnoresUnknownProjectTool(t *testing.T) {
	outcome := handleProjectMCPToolCall(mcpToolCall{Name: "not_project_tool", Arguments: map[string]any{}})
	if outcome.Handled {
		t.Fatalf("unknown project tool should be ignored: %#v", outcome)
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

func textFromMCPToolOutcome(outcome mcpToolOutcome) (string, error) {
	b, err := jsonMarshalIndentForMCPProjectTest(outcome.Payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonMarshalIndentForMCPProjectTest(payload any) ([]byte, error) {
	return json.MarshalIndent(payload, "", "  ")
}
