package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/loopgate"
	preflight "agent-harness/internal/adapter/preflight"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestRunIssueOpsRemoteVerifyArtifactValidationErrors(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	record, err := issueopscore.StartIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{
		Repo:   repo,
		Branch: "75-remote-verify-cli",
	})
	if err != nil {
		t.Fatal(err)
	}

	prePROut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"remote", "verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "pr", "--url", "https://github.com/example/repo/pull/1", "--label", "bug", "--assignee", "sample", "--json"})
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
			args: []string{"remote", "verify-artifact", "--id", record.ID, "--provider", "jira", "--kind", "pr", "--url", "https://github.com/example/repo/pull/1", "--label", "bug", "--assignee", "sample", "--json"},
			want: "remote artifact provider must be github or gitlab",
		},
		{
			name: "invalid kind",
			args: []string{"remote", "verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "mr", "--url", "https://github.com/example/repo/pull/1", "--label", "bug", "--assignee", "sample", "--json"},
			want: "github remote artifact kind must be pr",
		},
		{
			name: "missing labels",
			args: []string{"remote", "verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "pr", "--url", "https://github.com/example/repo/pull/1", "--assignee", "sample", "--json"},
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

func makeIssueOpsPRPhaseRecordForCLITest(t *testing.T, id, repo string) (issueopscontract.IssueOpsRecord, issueopscore.IssueOpsActor) {
	t.Helper()
	recordIssueOpsCoreIntentForCLITest(t, id)
	if _, err := issueopscore.LinkIssueOpsIssue(issueopscore.IssueOpsStateRoot(), id, "https://github.com/example/repo/issues/75"); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.PrepareIssueOpsBranch(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/75",
		Branch:       "75-remote-verify-cli",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", "75-remote-verify-cli"); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", "75-remote-verify-cli"); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "75-remote-verify-cli")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, "75-remote-verify-cli"); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	if _, err := issueopscore.LinkIssueOpsWorktree(issueopscore.IssueOpsStateRoot(), id, worktree); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsCoreDesignForCLITest(t, id)
	planPath := filepath.Join(worktree, "plans", "remote-verify.md")
	writeIssueOpsCLIFileForTest(t, worktree, "plans/remote-verify.md", "plan\n")
	if _, err := issueopscore.LinkIssueOpsPlan(issueopscore.IssueOpsStateRoot(), id, planPath); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.RecordIssueOpsCompatibilityReview(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"existing IssueOps JSON records remain readable"},
		SideEffects:           []string{"phase ordering changes are limited to IssueOps lifecycle gates"},
		RollbackPlan:          "Revert compatibility-review phase and readiness gate.",
		Verification:          []string{"compatibility review checked backward compatibility and side effects"},
		Approved:              true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.RecordIssueOpsDevilsAdvocateReview(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsCLIFileForTest(t, worktree, "internal/demo.go", "package demo\n")
	if code, _, stderr := preflight.GitCmd(worktree, "add", "plans/remote-verify.md", "internal/demo.go"); code != 0 {
		t.Fatalf("git add implementation failed: %s", stderr)
	}
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	_, actor := seedIssueOpsCLIExecution(t, record)
	if _, err := loopgate.AdvancePhaseWithActor(issueopscore.IssueOpsStateRoot(), id, string(issueopscore.IssueOpsPhaseAISlopClean), actor); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "commit", "-q", "-m", "feat: implement remote verify cli"); code != 0 {
		t.Fatalf("git commit implementation failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "push", "-q"); code != 0 {
		t.Fatalf("git push implementation failed: %s", stderr)
	}
	record, err = loopgate.AdvancePhaseWithActor(issueopscore.IssueOpsStateRoot(), id, string(issueopscore.IssueOpsPhasePR), actor)
	if err != nil {
		t.Fatal(err)
	}
	return record, actor
}

func makeIssueOpsCLIGitRepoForRemoteVerifyTest(t *testing.T) string {
	t.Helper()
	repo := makeIssueOpsCLIRepoForTest(t, "remote-verify-cli")
	remote := t.TempDir()
	if code, _, stderr := preflight.GitCmd(remote, "init", "--bare", "-q"); code != 0 {
		t.Fatalf("git init bare failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "init", "-q", "-b", "main"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Test"},
		{"config", "user.email", "issueops@example.test"},
		{"remote", "add", "origin", remote},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsCLIFileForTest(t, repo, "README.md", "readme\n")
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "initial"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", "main"); code != 0 {
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
