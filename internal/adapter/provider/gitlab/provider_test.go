package gitlab

import (
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitLabCreateIssueRequiresTitle(t *testing.T) {
	_, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Title: ""})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestGitLabCreateIssueDryRun(t *testing.T) {
	res, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{
		Title: "Fix bug",
		Body:  "details",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(res.Preview, "[dry-run]") {
		t.Errorf("expected dry-run preview, got %q", res.Preview)
	}
	if !strings.Contains(res.Preview, "issue create") || !strings.Contains(res.Preview, "--description details") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
}

func TestGitLabCreateMRRequiresBranches(t *testing.T) {
	_, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Title: "MR"})
	if err == nil {
		t.Fatal("expected error for missing source/target branches")
	}
}

func TestGitLabCreateMRDryRun(t *testing.T) {
	res, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{
		Title:      "Add feature",
		HeadBranch: "feat/x",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Preview, "mr create") || !strings.Contains(res.Preview, "--source-branch feat/x") || !strings.Contains(res.Preview, "--target-branch main") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
}

func TestParseGlabOutput(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantURL    string
		wantNumber string
	}{
		{"empty", "", "", ""},
		{"plain url", "creating...\nhttps://gitlab.com/g/p/-/issues/9\n", "https://gitlab.com/g/p/-/issues/9", ""},
		{"json with iid", `{"web_url":"https://gitlab.com/g/p/-/issues/9","iid":9}`, "https://gitlab.com/g/p/-/issues/9", "9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, number := parseGlabOutput(tc.in, "issue")
			if url != tc.wantURL || number != tc.wantNumber {
				t.Errorf("got url=%q number=%q, want url=%q number=%q", url, number, tc.wantURL, tc.wantNumber)
			}
		})
	}
}
