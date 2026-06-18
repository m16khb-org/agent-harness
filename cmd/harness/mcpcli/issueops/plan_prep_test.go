package issueops

import "testing"

func TestMCPIssueOpsPlanPrepRecord(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	out := callMCPToolForIssueOpsTest(t, "issueops_plan_prep_record", map[string]any{
		"id":                 id,
		"decisions_evidence": []string{".agent-harness/ADR.md"},
		"related_score_ref":  []string{"remote score: selected=#1(0.9), threshold=0.70"},
		"web_research_waive": "internal-only change",
	})
	pp, ok := out["plan_prep"].(map[string]any)
	if !ok {
		t.Fatalf("plan_prep should be persisted: %#v", out)
	}
	decisions, ok := pp["prior_decisions"].(map[string]any)
	if !ok || decisions["status"] != "evidence" {
		t.Fatalf("prior_decisions should carry evidence: %#v", pp)
	}
	research, ok := pp["web_research"].(map[string]any)
	if !ok || research["status"] != "waived" {
		t.Fatalf("web_research should be waived: %#v", pp)
	}
}

func TestMCPIssueOpsPlanPrepRejectsBothEvidenceAndWaive(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, _ := start["id"].(string)
	err := callMCPToolForIssueOpsTestError(t, "issueops_plan_prep_record", map[string]any{
		"id":                 id,
		"decisions_evidence": []string{"adr"},
		"decisions_waive":    "nope",
		"related_score_ref":  []string{"score"},
		"web_research_waive": "internal",
	})
	if err == nil {
		t.Fatal("evidence + waive on one item must error via MCP")
	}
}
