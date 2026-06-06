package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunIssueOpsWorktreeUsageAndErrorBranches(t *testing.T) {
	usage := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"worktree", "help"})
	})
	if !strings.Contains(usage, "Usage: agent-harness issueops worktree prepare-tools --id ID [--json]") {
		t.Fatalf("worktree usage missing prepare-tools syntax:\n%s", usage)
	}

	if err := runIssueOps([]string{"worktree", "bootstrap"}); err == nil || !strings.Contains(err.Error(), "unknown issueops worktree subcommand") {
		t.Fatalf("worktree unknown subcommand error = %v", err)
	}

	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	missingOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", "missing", "--json"})
	})
	if err == nil || !strings.Contains(err.Error(), "invalid issueops id") {
		t.Fatalf("missing record error = %v", err)
	}
	var missingPayload map[string]any
	if jsonErr := json.Unmarshal([]byte(missingOut), &missingPayload); jsonErr != nil {
		t.Fatalf("missing record should emit JSON error: %v\n%s", jsonErr, missingOut)
	}
	if missingPayload["ok"] != false || !strings.Contains(missingPayload["error"].(string), "invalid issueops id") {
		t.Fatalf("unexpected missing record payload: %#v", missingPayload)
	}

	repo := makeIssueOpsCLIRepoForTest(t, "worktree-missing-path")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "71-worktree-missing-path", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id := record["id"].(string)
	pathOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", id, "--json"})
	})
	if err == nil || !strings.Contains(err.Error(), "worktree_path is required") {
		t.Fatalf("missing worktree path error = %v", err)
	}
	var pathPayload map[string]any
	if jsonErr := json.Unmarshal([]byte(pathOut), &pathPayload); jsonErr != nil {
		t.Fatalf("missing worktree path should emit JSON error: %v\n%s", jsonErr, pathOut)
	}
	if pathPayload["ok"] != false || !strings.Contains(pathPayload["error"].(string), "worktree_path is required") {
		t.Fatalf("unexpected missing worktree path payload: %#v", pathPayload)
	}
}

func TestIssueOpsWorktreePackageManagerBranches(t *testing.T) {
	root := t.TempDir()
	if got := issueOpsWorktreePackageManager(root); got != "" {
		t.Fatalf("package manager without package.json = %q", got)
	}

	writeIssueOpsCLIFileForTest(t, root, "package.json", "{}")
	if got := issueOpsWorktreePackageManager(root); got != "" {
		t.Fatalf("package manager without lockfile = %q", got)
	}

	writeIssueOpsCLIFileForTest(t, root, "yarn.lock", "")
	if got := issueOpsWorktreePackageManager(root); got != "yarn" {
		t.Fatalf("package manager with yarn.lock = %q", got)
	}

	npmRoot := filepath.Join(t.TempDir(), "npm")
	writeIssueOpsCLIFileForTest(t, npmRoot, "package.json", "{}")
	writeIssueOpsCLIFileForTest(t, npmRoot, "package-lock.json", "{}")
	if got := issueOpsWorktreePackageManager(npmRoot); got != "npm" {
		t.Fatalf("package manager with package-lock.json = %q", got)
	}
}
