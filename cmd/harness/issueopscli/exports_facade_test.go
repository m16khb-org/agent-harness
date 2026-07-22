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
}

func TestIssueOpsSubcommandSuggestions(t *testing.T) {
	cases := []struct {
		input        string
		mustContain  string
		mustNotMatch bool
	}{
		// Concept hints: domain vocabulary mistaken for subcommands.
		{"grill", "issueops phase --to grill", false},
		{"split", "issueops remote create-child", false},
		{"problem", "issueops phase --to problem", false},
		{"implement", "issueops phase --to implement", false},
		// Prefix matches against the real registry.
		{"domain", "domain-review", false},
		{"compat", "compatibility", false},
		{"execut", "execution", false},
		// No suggestion for garbage input — bare error only.
		{"totally-bogus", "", true},
	}
	for _, tc := range cases {
		err := RunIssueOps([]string{tc.input})
		if err == nil {
			t.Fatalf("input %q should fail", tc.input)
		}
		msg := err.Error()
		if !strings.Contains(msg, `unknown issueops subcommand`) {
			t.Fatalf("input %q: error missing canonical prefix: %q", tc.input, msg)
		}
		if tc.mustNotMatch {
			if strings.Contains(msg, "did you mean") {
				t.Fatalf("input %q: should have no suggestion, got %q", tc.input, msg)
			}
			continue
		}
		if !strings.Contains(msg, "did you mean") || !strings.Contains(msg, tc.mustContain) {
			t.Fatalf("input %q: expected suggestion containing %q, got %q", tc.input, tc.mustContain, msg)
		}
	}
}
