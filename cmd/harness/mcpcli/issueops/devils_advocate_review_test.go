package issueops

import (
	"testing"
)

func TestIssueOpsMCPRecordsDevilsAdvocateReview(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	configureIssueOpsMCPForTest(t)
	started := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{
		"repo":   makeIssueOpsCLIRepoForTest(t, "mcp-devils-advocate"),
		"branch": "126-devils-advocate",
	})
	id := started["id"].(string)

	pass := callMCPToolForIssueOpsTest(t, "issueops_record_devils_advocate_review", map[string]any{
		"id":      id,
		"verdict": "pass",
	})
	review, ok := pass["devils_advocate_review"].(map[string]any)
	if !ok || review["verdict"] != "pass" || review["reviewer_pattern"] != "devils-advocate-review" {
		t.Fatalf("MCP devils-advocate review not persisted: %#v", pass)
	}

	// A stop verdict needs findings or a waiver.
	rejected := callMCPToolForIssueOpsTestError(t, "issueops_record_devils_advocate_review", map[string]any{
		"id":      id,
		"verdict": "stop",
	})
	if rejected == "" {
		t.Fatal("stop without findings/waiver should be rejected")
	}
}
