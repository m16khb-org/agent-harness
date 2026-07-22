package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunIssueOpsRemoteVerifyArtifactValidationErrors(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{
		Repo:   repo,
		Branch: "75-remote-verify-cli",
	})
	if err != nil {
		t.Fatal(err)
	}

	prePROut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"remote", "verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "pr", "--url", "https://github.com/example/repo/pull/1", "--label", "bug", "--assignee", "habin", "--json"})
	})
	assertIssueOpsJSONErrorContains(t, prePROut, err, "cannot verify remote artifact before pr phase")

	record, actor := makeIssueOpsPRPhaseRecordForCLITest(t, record.ID, repo)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "invalid provider",
			args: []string{"remote", "verify-artifact", "--id", record.ID, "--provider", "jira", "--kind", "pr", "--url", "https://github.com/example/repo/pull/1", "--label", "bug", "--assignee", "habin", "--json"},
			want: "remote artifact provider must be github or gitlab",
		},
		{
			name: "invalid kind",
			args: []string{"remote", "verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "mr", "--url", "https://github.com/example/repo/pull/1", "--label", "bug", "--assignee", "habin", "--json"},
			want: "github remote artifact kind must be pr",
		},
		{
			name: "missing labels",
			args: []string{"remote", "verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "pr", "--url", "https://github.com/example/repo/pull/1", "--assignee", "habin", "--json"},
			want: "remote artifact labels are required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := captureStdoutAndErrorForIssueOps(t, func() error {
				return runIssueOps(withIssueOpsCLIActor(tc.args, actor))
			})
			assertIssueOpsJSONErrorContains(t, out, err, tc.want)
		})
	}
}

func makeIssueOpsPRPhaseRecordForCLITest(t *testing.T, id, repo string) (core.IssueOpsRecord, core.IssueOpsActor) {
	t.Helper()
	recordIssueOpsCoreIntentForCLITest(t, id)
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), id, "https://github.com/example/repo/issues/75"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), id, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/75",
		Branch:       "75-remote-verify-cli",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := core.GitCmd(repo, "checkout", "-q", "-b", "75-remote-verify-cli"); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := core.GitCmd(repo, "push", "-q", "-u", "origin", "75-remote-verify-cli"); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := core.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "75-remote-verify-cli")
	if code, _, stderr := core.GitCmd(repo, "worktree", "add", "-q", worktree, "75-remote-verify-cli"); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	if _, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), id, worktree); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsCoreDesignForCLITest(t, id)
	planPath := filepath.Join(worktree, "plans", "remote-verify.md")
	writeIssueOpsCLIFileForTest(t, worktree, "plans/remote-verify.md", "plan\n")
	if _, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), id, planPath); err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordIssueOpsCompatibilityReview(core.IssueOpsStateRoot(), id, core.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"existing IssueOps JSON records remain readable"},
		SideEffects:           []string{"phase ordering changes are limited to IssueOps lifecycle gates"},
		RollbackPlan:          "Revert compatibility-review phase and readiness gate.",
		Verification:          []string{"compatibility review checked backward compatibility and side effects"},
		Approved:              true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordIssueOpsDevilsAdvocateReview(core.IssueOpsStateRoot(), id, core.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsCLIFileForTest(t, worktree, "internal/demo.go", "package demo\n")
	if code, _, stderr := core.GitCmd(worktree, "add", "plans/remote-verify.md", "internal/demo.go"); code != 0 {
		t.Fatalf("git add implementation failed: %s", stderr)
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record, actor := seedIssueOpsCLIExecutionV1(t, record)
	if _, err := core.AdvanceIssueOpsPhaseWithActor(core.IssueOpsStateRoot(), id, string(core.IssueOpsPhaseAISlopClean), actor); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := core.GitCmd(worktree, "commit", "-q", "-m", "feat: implement remote verify cli"); code != 0 {
		t.Fatalf("git commit implementation failed: %s", stderr)
	}
	if code, _, stderr := core.GitCmd(worktree, "push", "-q"); code != 0 {
		t.Fatalf("git push implementation failed: %s", stderr)
	}
	record, err = core.AdvanceIssueOpsPhaseWithActor(core.IssueOpsStateRoot(), id, string(core.IssueOpsPhasePR), actor)
	if err != nil {
		t.Fatal(err)
	}
	return record, actor
}

func makeIssueOpsCLIGitRepoForRemoteVerifyTest(t *testing.T) string {
	t.Helper()
	repo := makeIssueOpsCLIRepoForTest(t, "remote-verify-cli")
	remote := t.TempDir()
	if code, _, stderr := core.GitCmd(remote, "init", "--bare", "-q"); code != 0 {
		t.Fatalf("git init bare failed: %s", stderr)
	}
	if code, _, stderr := core.GitCmd(repo, "init", "-q", "-b", "main"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Test"},
		{"config", "user.email", "issueops@example.test"},
		{"remote", "add", "origin", remote},
	} {
		if code, _, stderr := core.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsCLIFileForTest(t, repo, "README.md", "readme\n")
	if code, _, stderr := core.GitCmd(repo, "add", "README.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := core.GitCmd(repo, "commit", "-q", "-m", "initial"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := core.GitCmd(repo, "push", "-q", "-u", "origin", "main"); code != 0 {
		t.Fatalf("git push main failed: %s", stderr)
	}
	return repo
}

func assertIssueOpsJSONErrorContains(t *testing.T, out string, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
	var payload map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &payload); jsonErr != nil {
		t.Fatalf("expected JSON error output: %v\n%s", jsonErr, out)
	}
	if payload["ok"] != false || !strings.Contains(payload["error"].(string), want) {
		t.Fatalf("unexpected JSON error payload: %#v", payload)
	}
}
