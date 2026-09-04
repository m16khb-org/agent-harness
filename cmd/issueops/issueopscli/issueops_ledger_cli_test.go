package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"
)

func startLedgerCLICycle(t *testing.T) string {
	t.Helper()
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-ledger", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start JSON: %v\n%s", err, start)
	}
	id, _ := record["id"].(string)
	if id == "" {
		t.Fatalf("start should return id: %#v", record)
	}
	return id
}

func TestRunIssueOpsDomainReviewRecord(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	id := startLedgerCLICycle(t)
	if err := runIssueOps([]string{"domain-review", "record", "--id", id}); err == nil {
		t.Fatal("domain-review without model-fit/terminology must error")
	}
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"domain-review", "record", "--id", id, "--model-fit", "fits the model", "--terminology", "ledger", "--risk", "deadlock", "--json"})
	})
	if !strings.Contains(out, "domain_review") || !strings.Contains(out, "fits the model") {
		t.Fatalf("domain-review record should emit domain_review: %s", out)
	}
}

func TestRunIssueOpsAISlopCleanRecord(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	id := startLedgerCLICycle(t)
	if err := runIssueOps([]string{"ai-slop-clean", "record", "--id", id, "--category", "dead-code"}); err == nil {
		t.Fatal("ai-slop-clean record without verification must error")
	}
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"ai-slop-clean", "record", "--id", id, "--category", "dead-code", "--verification", "go test ./...", "--json"})
	})
	if !strings.Contains(out, "ai_slop_clean_categories") || !strings.Contains(out, "dead-code") {
		t.Fatalf("ai-slop-clean record should emit categories: %s", out)
	}
}

func TestRunIssueOpsFeedbackResolve(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	id := startLedgerCLICycle(t)
	if err := runIssueOps([]string{"feedback", "add", "--id", id, "--source", "review", "--body", "fix the bug", "--classification", "defect"}); err != nil {
		t.Fatalf("add feedback: %v", err)
	}
	if err := runIssueOps([]string{"feedback", "resolve", "--id", id, "--index", "0", "--resolution", "bogus"}); err == nil {
		t.Fatal("bogus resolution must error")
	}
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"feedback", "resolve", "--id", id, "--index", "0", "--resolution", "valid-defect", "--json"})
	})
	if !strings.Contains(out, "valid-defect") {
		t.Fatalf("feedback resolve should emit resolution: %s", out)
	}
}
