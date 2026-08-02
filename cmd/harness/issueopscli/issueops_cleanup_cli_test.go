package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"

	"agent-harness/internal/core"
)

func TestRunIssueOpsUsageAndCleanupBranches(t *testing.T) {
	stderr, err := captureProjectCLIStderr(t, func() error {
		return runIssueOps(nil)
	})
	if err != nil {
		t.Fatalf("issueops usage failed: %v", err)
	}
	for _, want := range []string{
		"Usage:",
		"agent-harness issueops cleanup status --id ID [--merged] [--json]",
		"agent-harness issueops cleanup orphan --id ID --repo ROOT --worktree PATH --branch NAME",
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
	if !strings.Contains(cleanupUsage, "agent-harness issueops cleanup orphan --id ID --repo ROOT --worktree PATH --branch NAME") {
		t.Fatalf("cleanup usage missing recordless orphan syntax:\n%s", cleanupUsage)
	}

	designUsage := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"design", "review", "--help"})
	})
	for _, want := range []string{
		"Approved reviews require --refactor-plan, at least one --alternative, at least one --risk",
		`--verification "design review checked alternatives and risks"`,
		"Approval is recorded with the full design review payload; there is no approve-only merge step.",
	} {
		if !strings.Contains(designUsage, want) {
			t.Fatalf("design review usage missing %q:\n%s", want, designUsage)
		}
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

func TestRunIssueOpsCleanupCloseChildrenRequiresMergedAndConfirmRecordsState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bin := t.TempDir()
	writeFakeGhForCloseChildren(t, bin)
	t.Setenv("PATH", bin)
	record := issueopscontract.IssueOpsRecord{
		ID:       core.NewIssueOpsID(repo, "12-child-cleanup"),
		Repo:     repo,
		Branch:   "12-child-cleanup",
		Phase:    core.IssueOpsPhasePR,
		IssueURL: "https://github.com/acme/repo/issues/12",
		IssueLinks: []issueopscontract.IssueOpsIssueLink{{
			Type:     "child",
			URL:      "https://github.com/acme/repo/issues/34",
			Provider: "github",
		}},
	}
	record.RemoteArtifact = &issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/acme/repo/pull/55",
		Labels:    []string{"issueops"},
		Assignees: []string{"octocat"},
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	if err := runIssueOps([]string{"cleanup", "close-children", "--id", record.ID, "--json"}); err == nil || !strings.Contains(err.Error(), "merge evidence") {
		t.Fatalf("close-children should require merge evidence, got %v", err)
	}

	jsonOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"cleanup", "close-children", "--id", record.ID, "--merged", "--confirm", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil {
		t.Fatalf("close-children should return JSON: %v\n%s", err, jsonOut)
	}
	if result["closed_count"] != float64(1) || result["dry_run"] == true {
		t.Fatalf("unexpected close-children result: %#v", result)
	}
	updated, err := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.IssueLinks[0].CloseVerifiedAt == "" || updated.IssueLinks[0].CloseReason != "completed" {
		t.Fatalf("close evidence not recorded: %+v", updated.IssueLinks[0])
	}
}

func writeFakeGhForCloseChildren(t *testing.T, binDir string) {
	t.Helper()
	script := `#!/bin/sh
if [ "$1 $2" = "pr view" ]; then
  printf '{"url":"https://github.com/acme/repo/pull/55","state":"MERGED","mergedAt":"2026-06-17T00:00:00Z","labels":[{"name":"issueops"}],"assignees":[{"login":"octocat"}]}'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/12/sub_issues" ]; then
  printf '[{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"open"}]'
  exit 0
fi
if [ "$1 $2" = "api -X" ] && [ "$3" = "PATCH" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"closed"}'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/34" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"closed"}'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
