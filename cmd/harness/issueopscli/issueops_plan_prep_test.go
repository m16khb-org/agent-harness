package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunIssueOpsPlanPrepRecord(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start JSON: %v\n%s", err, start)
	}
	id, _ := record["id"].(string)
	if id == "" {
		t.Fatalf("start should return id: %#v", record)
	}

	// Evidence and waive on the same item must be rejected.
	if err := runIssueOps([]string{
		"plan-prep", "record", "--id", id,
		"--decisions-evidence", "adr", "--decisions-waive", "nope",
		"--related-score-ref", "score", "--web-research-waive", "internal",
	}); err == nil {
		t.Fatal("evidence + waive on one item must error")
	}

	// A valid mix of evidence and waive succeeds and emits plan_prep.
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"plan-prep", "record", "--id", id,
			"--decisions-evidence", ".agent-harness/ADR.md",
			"--related-score-ref", "remote score: selected=#1(0.9), threshold=0.70",
			"--web-research-waive", "internal-only change",
			"--json",
		})
	})
	if !strings.Contains(out, "plan_prep") || !strings.Contains(out, "waived") {
		t.Fatalf("plan-prep record should emit plan_prep with a waived item: %s", out)
	}
}
