package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestIssueOpsCompatibilityReviewCLIRecordsReview(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "123-compatibility-review", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id := record["id"].(string)
	recordIssueOpsCoreIntentForCLITest(t, id)
	if _, err := issueopscore.LinkIssueOpsIssue(issueopscore.IssueOpsStateRoot(), id, "https://github.com/example/repo/issues/123"); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.PrepareIssueOpsBranch(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/123",
		Branch:       "123-compatibility-review",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := makeIssueOpsCLIWorktreeForTest(t, repo, "123-compatibility-review")
	if _, err := issueopscore.LinkIssueOpsWorktree(issueopscore.IssueOpsStateRoot(), id, worktree); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsCoreDesignForCLITest(t, id)
	writeIssueOpsCLIFileForTest(t, worktree, "plans/demo.md", "plan\n")
	if _, err := issueopscore.LinkIssueOpsPlan(issueopscore.IssueOpsStateRoot(), id, filepath.Join(worktree, "plans/demo.md")); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"compatibility", "review",
			"--id", id,
			"--backward-compatibility", "existing IssueOps JSON records remain readable",
			"--side-effect", "phase ordering changes are limited to IssueOps lifecycle gates",
			"--rollback-plan", "revert the compatibility-review phase and readiness gate",
			"--verification", "compatibility review checked backward compatibility and side effects",
			"--approved",
			"--json",
		})
	})
	var updated map[string]any
	if err := json.Unmarshal([]byte(out), &updated); err != nil {
		t.Fatalf("compatibility review should return JSON: %v\n%s", err, out)
	}
	review, ok := updated["compatibility_review"].(map[string]any)
	if !ok || review["approved"] != true || review["reviewed_at"] == "" {
		t.Fatalf("compatibility review not persisted in CLI output: %#v", updated)
	}
	if updated["phase"] != "compatibility-review" {
		t.Fatalf("compatibility review should move cycle to compatibility-review phase: %#v", updated)
	}
}
