package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsExecutionDecideCLIRecordsDecision(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "123-execution-decision", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id := record["id"].(string)

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"execution", "decide",
			"--id", id,
			"--auto", "continue from plan to implement when readiness is green",
			"--hook-block", "hooks do not create issues or decide sub-agent usage",
			"--human-gate", "ask before destructive cleanup",
			"--subagent-use", "none",
			"--subagent-rationale", "main agent owns this focused implementation",
			"--json",
		})
	})
	var updated map[string]any
	if err := json.Unmarshal([]byte(out), &updated); err != nil {
		t.Fatalf("execution decide should return JSON: %v\n%s", err, out)
	}
	decision, ok := updated["execution_decision"].(map[string]any)
	if !ok || decision["subagent_use"] != "none" || decision["recorded_at"] == "" {
		t.Fatalf("execution decision not persisted in CLI output: %#v", updated)
	}
}

func TestIssueOpsExecutionDecideCLIRejectsUnknownPlanFileFields(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "124-execution-plan-file", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id := record["id"].(string)
	planFile := filepath.Join(t.TempDir(), "subagents.json")
	if err := os.WriteFile(planFile, []byte(`[{"objective":"review","pattern":"devils-advocate-review","benefit":"fresh_review","tradeoffs":["cannot steer mid-run"],"net_positive_rationale":"fresh review outweighs overhead","scope":"changed files","verification":"findings with evidence","fallback":"main agent reviews","extra":"reject"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{
			"execution", "decide",
			"--id", id,
			"--auto", "continue from plan to implement when readiness is green",
			"--hook-block", "hooks do not create issues or decide sub-agent usage",
			"--human-gate", "ask before destructive cleanup",
			"--subagent-use", "planned",
			"--subagent-plan-file", planFile,
			"--json",
		})
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown subagent plan field should fail strict decode, err=%v out=%s", err, out)
	}
}
