package issueops

import (
	"testing"
)

func TestIssueOpsMCPLedgerRecorders(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	configureIssueOpsMCPForTest(t)
	started := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{
		"repo":   makeIssueOpsCLIRepoForTest(t, "mcp-ledger"),
		"branch": "1-ledger",
	})
	id := started["id"].(string)

	domain := callMCPToolForIssueOpsTest(t, "issueops_record_domain_review", map[string]any{
		"id":          id,
		"model_fit":   "fits the phase model",
		"terminology": []any{"ledger"},
		"risks":       []any{"deadlock"},
	})
	review, ok := domain["domain_review"].(map[string]any)
	if !ok || review["model_fit"] != "fits the phase model" {
		t.Fatalf("MCP domain review not persisted: %#v", domain)
	}

	evidence := callMCPToolForIssueOpsTest(t, "issueops_record_ai_slop_clean_evidence", map[string]any{
		"id":           id,
		"categories":   []any{"dead-code"},
		"verification": []any{"go test ./..."},
	})
	if _, ok := evidence["ai_slop_clean_categories"]; !ok {
		t.Fatalf("MCP cleanup categories not persisted: %#v", evidence)
	}

	callMCPToolForIssueOpsTest(t, "issueops_add_feedback", map[string]any{"id": id, "source": "review", "body": "fix the bug", "classification": "defect"})
	resolved := callMCPToolForIssueOpsTest(t, "issueops_resolve_feedback", map[string]any{"id": id, "index": 0, "resolution": "valid-defect"})
	feedback, ok := resolved["feedback"].([]any)
	if !ok || len(feedback) == 0 {
		t.Fatalf("feedback missing after resolve: %#v", resolved)
	}
	if first, _ := feedback[0].(map[string]any); first["resolution"] != "valid-defect" {
		t.Fatalf("MCP feedback resolution not persisted: %#v", feedback[0])
	}
}
