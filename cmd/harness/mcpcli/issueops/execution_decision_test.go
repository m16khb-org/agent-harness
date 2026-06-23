package issueops

import (
	"testing"
)

func TestIssueOpsMCPRecordsExecutionDecision(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	configureIssueOpsMCPForTest(t)
	started := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{
		"repo":   makeIssueOpsCLIRepoForTest(t, "mcp-execution"),
		"branch": "125-execution-decision",
	})
	id := started["id"].(string)

	payload := callMCPToolForIssueOpsTest(t, "issueops_record_execution_decision", map[string]any{
		"id":                 id,
		"auto_proceed":       []any{"implementation may proceed after durable readiness gates"},
		"hook_blocked":       []any{"hooks do not create issues or choose sub-agents"},
		"human_gates":        []any{"ask before destructive cleanup"},
		"subagent_use":       "planned",
		"subagent_rationale": "fresh review is useful after implementation",
		"subagent_plans": []any{map[string]any{
			"objective":              "review the final diff",
			"pattern":                "devils-advocate-review",
			"benefit":                "fresh_review",
			"tradeoffs":              []any{"cannot steer the reviewer mid-run", "adds latency and tokens"},
			"net_positive_rationale": "fresh-context review is worth the overhead because the main agent authored the diff",
			"scope":                  "changed IssueOps files only",
			"verification":           "report file and line evidence",
			"fallback":               "main agent reviews directly",
		}},
	})
	decision, ok := payload["execution_decision"].(map[string]any)
	if !ok || decision["subagent_use"] != "planned" {
		t.Fatalf("MCP execution decision not persisted: %#v", payload)
	}
}
