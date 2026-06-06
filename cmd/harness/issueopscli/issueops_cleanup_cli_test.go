package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunIssueOpsUsageAndCleanupBranches(t *testing.T) {
	stderr, err := captureProjectCLIStderr(func() error {
		return runIssueOps(nil)
	})
	if err != nil {
		t.Fatalf("issueops usage failed: %v", err)
	}
	for _, want := range []string{
		"Usage:",
		"agent-harness issueops cleanup status --id ID [--merged] [--json]",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("issueops usage missing %q:\n%s", want, stderr)
		}
	}

	cleanupUsage := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"cleanup", "help"})
	})
	if !strings.Contains(cleanupUsage, "Usage: agent-harness issueops cleanup status --id ID [--merged] [--json]") {
		t.Fatalf("cleanup usage missing status syntax:\n%s", cleanupUsage)
	}

	if err := runIssueOps([]string{"cleanup", "remove"}); err == nil || !strings.Contains(err.Error(), "unknown issueops cleanup subcommand") {
		t.Fatalf("cleanup unknown subcommand error = %v", err)
	}
}

func TestRunIssueOpsCleanupStatusTextAndJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "cleanup-cli")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "99-cleanup-cli", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	text := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"cleanup", "status", "--id", id})
	})
	for _, want := range []string{
		"ready: false",
		"- missing:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("cleanup text missing %q:\n%s", want, text)
		}
	}

	jsonOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"cleanup", "status", "--id", id, "--json"})
	})
	var status map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &status); err != nil {
		t.Fatalf("cleanup status should return JSON: %v\n%s", err, jsonOut)
	}
	if status["ready"] == true {
		t.Fatalf("new issueops record should not be cleanup-ready: %#v", status)
	}
	if missing, ok := status["missing"].([]any); !ok || len(missing) == 0 {
		t.Fatalf("cleanup status should report missing gates: %#v", status)
	}
}
