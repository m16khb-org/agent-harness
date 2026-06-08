package github

import (
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitHubCreateIssueRequiresTitle(t *testing.T) {
	_, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Title: "  "})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestGitHubCreateIssueDryRunDoesNotExecute(t *testing.T) {
	res, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{
		Title:  "Fix bug",
		Body:   "details",
		Labels: []string{"bug"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Fatal("expected OK dry-run result")
	}
	if !strings.HasPrefix(res.Preview, "[dry-run]") {
		t.Errorf("expected dry-run preview, got %q", res.Preview)
	}
	if !strings.Contains(res.Preview, "issue create") || !strings.Contains(res.Preview, "--label bug") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
	if res.URL != "" || res.Number != "" {
		t.Error("dry-run must not populate URL/Number")
	}
}

func TestGitHubCreatePullRequestRequiresBranches(t *testing.T) {
	_, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Title: "PR"})
	if err == nil {
		t.Fatal("expected error for missing head/base branches")
	}
}

func TestGitHubCreatePullRequestDryRun(t *testing.T) {
	res, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{
		Title:      "Add feature",
		HeadBranch: "feat/x",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Preview, "pr create") || !strings.Contains(res.Preview, "--head feat/x") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
}

func TestParseGhOutput(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantURL    string
		wantNumber string
	}{
		{"empty", "", "", ""},
		{"plain url", "https://github.com/o/r/issues/12\n", "https://github.com/o/r/issues/12", ""},
		{"json", `{"url":"https://github.com/o/r/pull/7","number":"7"}`, "https://github.com/o/r/pull/7", "7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGhOutput(tc.in, "issue")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.URL != tc.wantURL || got.Number != tc.wantNumber {
				t.Errorf("got %+v, want url=%q number=%q", got, tc.wantURL, tc.wantNumber)
			}
		})
	}
}
