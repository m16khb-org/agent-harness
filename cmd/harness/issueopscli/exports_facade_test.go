package issueopscli

import (
	"errors"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestExportedIssueOpsFacades(t *testing.T) {
	if err := RunIssueOps([]string{"unknown"}); err == nil {
		t.Fatal("unknown issueops subcommand should fail")
	}
	if CleanupMerged("", false) {
		t.Fatal("cleanup without id and request should not be treated as merged")
	}
	if _, err := PrepareWorktreeTools(core.IssueOpsRecord{}); err == nil {
		t.Fatal("record without worktree context should not prepare tools")
	}
	if err := VerifyRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "pr", URL: "not-a-url"}); err == nil {
		t.Fatal("invalid remote artifact URL should fail before provider inspection")
	}

	sentinel := errors.New("sentinel")
	previous := SetChildIssueVerifier(func(string) error { return sentinel })
	defer SetChildIssueVerifier(previous)
	if err := VerifyChildIssueBeforeLink("https://github.com/acme/repo/issues/1"); !errors.Is(err, sentinel) {
		t.Fatalf("stubbed child verifier err=%v", err)
	}
}

func TestIssueOpsBenchmarkArtifactFacades(t *testing.T) {
	fixture := core.IssueOpsBenchmarkFixture{
		Title:         "Fix quality gate",
		UserPrompt:    "raise coverage",
		RepoContext:   "agent-harness",
		ExpectedIssue: []string{"quality label"},
		ExpectedTasks: []string{"add tests"},
	}
	artifact := benchmarkArtifactFromFixture(fixture)
	if !strings.Contains(artifact.ProblemSummary, "raise coverage") || !strings.Contains(artifact.IssueDraft, "quality label") {
		t.Fatalf("artifact = %#v", artifact)
	}
	if bullets := issueOpsBenchmarkBullets([]string{"one", "two"}); !strings.Contains(bullets, "- one") || !strings.Contains(bullets, "- two") {
		t.Fatalf("bullets = %q", bullets)
	}
	if tasks := issueOpsBenchmarkOwnedTasks([]string{"add tests"}); !strings.Contains(tasks, "owns add tests") {
		t.Fatalf("owned tasks = %q", tasks)
	}
}

func TestIssueOpsDecisionAndCleanupCLIBranches(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "123-decision"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runIssueOpsDecision(nil); err != nil {
		t.Fatalf("decision help: %v", err)
	}
	if err := runIssueOpsDecision([]string{"remove"}); err == nil {
		t.Fatal("unknown decision subcommand should fail")
	}
	if err := runIssueOpsDecision([]string{
		"add",
		"--id", record.ID,
		"--title", "Use focused tests",
		"--body", "Raise low coverage with boundary tests",
		"--kind", "test",
		"--rationale", "quality gate",
		"--alternative", "change threshold",
		"--affected-artifact", "test",
		"--json",
	}); err != nil {
		t.Fatalf("decision add: %v", err)
	}
	if err := runIssueOpsCleanupStale([]string{"--prune-done=bad"}); err == nil {
		t.Fatal("invalid prune duration should fail")
	}
	if err := runIssueOpsCleanupStale([]string{"--repo", ""}); err == nil {
		t.Fatal("missing repo should fail in text/json result")
	}
	if err := runIssueOpsCleanup([]string{"stale", "--repo", t.TempDir(), "--max-age", "1"}); err != nil {
		t.Fatalf("cleanup stale dry run: %v", err)
	}
}
