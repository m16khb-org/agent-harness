package remoteverify

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

// installFakeGHIssueForRemoteArtifactTest installs a fake gh whose `issue view`
// answers by issue number embedded in the URL ($3), mirroring the live PR fake
// but for the issue-verify path.
func installFakeGHIssueForRemoteArtifactTest(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	gh := filepath.Join(bin, "gh")
	script := `#!/bin/sh
case "$3" in
  *"/issues/1")
    printf '%s\n' '{"url":"https://github.com/example/repo/issues/1","labels":[{"name":"bug"}],"assignees":[{"login":"sample","name":"Habin"}],"state":"OPEN"}'
    exit 0
    ;;
  *)
    echo "not found" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(gh, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestVerifyIssueOpsRemoteArtifactLiveRejectsMissingGitHubIssue(t *testing.T) {
	installFakeGHIssueForRemoteArtifactTest(t)
	err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "issue",
		URL:       "https://github.com/example/repo/issues/9999",
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing GitHub issue to fail live verification, got %v", err)
	}
}

func TestVerifyIssueOpsRemoteArtifactLiveRequiresGitHubIssueLabelsAndAssignees(t *testing.T) {
	installFakeGHIssueForRemoteArtifactTest(t)
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "issue",
		URL:       "https://github.com/example/repo/issues/1",
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	}); err != nil {
		t.Fatalf("expected matching GitHub issue evidence to pass: %v", err)
	}
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "issue",
		URL:       "https://github.com/example/repo/issues/1",
		Labels:    []string{"missing"},
		Assignees: []string{"sample"},
	}); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("expected missing label to fail live verification, got %v", err)
	}
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "issue",
		URL:       "https://github.com/example/repo/issues/1",
		Labels:    []string{"bug"},
		Assignees: []string{"other"},
	}); err == nil || !strings.Contains(err.Error(), "assignee") {
		t.Fatalf("expected missing assignee to fail live verification, got %v", err)
	}
}

func TestFetchGitLabIssueArtifactParsesLivePayload(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "glab.log")
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' "$*" > "$ISSUEOPS_FAKE_GLAB_LOG"
printf '%s\n' '{"web_url":"https://gitlab.example.com/group/project/-/issues/42","state":"opened","labels":["ready","bug"],"assignees":[{"id":123,"username":"sample","name":"Ha Bin"},{"id":0,"username":"reviewer","name":"Reviewer"}]}'
`)
	t.Setenv("ISSUEOPS_FAKE_GLAB_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	artifact, err := fetchGitLabIssueArtifact("https://gitlab.example.com/group/project/-/issues/42")
	if err != nil {
		t.Fatalf("fetch GitLab issue artifact: %v", err)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := "api projects/group%2Fproject/issues/42 --hostname gitlab.example.com"
	if got := strings.TrimSpace(string(log)); got != wantArgs {
		t.Fatalf("glab args = %q, want %q", got, wantArgs)
	}
	if artifact.URL != "https://gitlab.example.com/group/project/-/issues/42" {
		t.Fatalf("unexpected artifact identity: %#v", artifact)
	}
	for _, want := range []string{"ready", "bug"} {
		if !slices.Contains(artifact.Labels, want) {
			t.Fatalf("missing label %q in %#v", want, artifact.Labels)
		}
	}
	for _, want := range []string{"123", "sample", "Ha Bin", "reviewer", "Reviewer"} {
		if !slices.Contains(artifact.Assignees, want) {
			t.Fatalf("missing assignee %q in %#v", want, artifact.Assignees)
		}
	}
}

func TestVerifyIssueOpsRemoteArtifactLiveRequiresGitLabIssueLabelsAndAssignees(t *testing.T) {
	bin := t.TempDir()
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' '{"web_url":"https://gitlab.example.com/group/project/-/issues/42","state":"opened","labels":["bug"],"assignees":[{"id":1,"username":"sample","name":"Habin"}]}'
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "issue",
		URL:       "https://gitlab.example.com/group/project/-/issues/42",
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	}); err != nil {
		t.Fatalf("expected matching GitLab issue evidence to pass: %v", err)
	}
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "issue",
		URL:       "https://gitlab.example.com/group/project/-/issues/42",
		Labels:    []string{"missing"},
		Assignees: []string{"sample"},
	}); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("expected missing label to fail live verification, got %v", err)
	}
}

func TestFetchGitLabIssueArtifactRejectsNonIssueURL(t *testing.T) {
	_, err := fetchGitLabIssueArtifact("https://gitlab.example.com/group/project/-/merge_requests/42")
	if err == nil || !strings.Contains(err.Error(), "remote artifact url must be a GitLab issue URL") {
		t.Fatalf("expected invalid issue URL error, got %v", err)
	}
}

// GitLab 18.10+(관측 19.2.4-ee)은 일반 이슈를 /-/work_items/:iid로 렌더하고 glab
// issue create는 그 web_url을 그대로 출력한다. create-issue --confirm의 라이브 게이트는 원격 이슈가
// 이미 만들어진 뒤에 도는 검증이므로, 별칭을 거부하지 말고 issues/:iid endpoint로
// 해석해야 한다(2026-08-26 lesson).
func TestFetchGitLabIssueArtifactAcceptsWorkItemsAlias(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "glab.log")
	// fail-closed fake: issues/42 REST endpoint 이외의 호출은 실패한다.
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' "$*" >> "$ISSUEOPS_FAKE_GLAB_LOG"
case "$*" in
  "api projects/group%2Fproject/issues/42 --hostname gitlab.example.com")
    printf '%s\n' '{"web_url":"https://gitlab.example.com/group/project/-/work_items/42","state":"opened","labels":["bug"],"assignees":[{"id":1,"username":"sample","name":"Habin"}]}'
    ;;
  *)
    echo "unexpected glab call: $*" >&2
    exit 1
    ;;
esac
`)
	t.Setenv("ISSUEOPS_FAKE_GLAB_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	issueURL := "https://gitlab.example.com/group/project/-/work_items/42"
	artifact, err := fetchGitLabIssueArtifact(issueURL)
	if err != nil {
		t.Fatalf("work_items alias must resolve on the issues endpoint: %v", err)
	}
	if artifact.URL != issueURL {
		t.Fatalf("unexpected artifact identity: %#v", artifact)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := "api projects/group%2Fproject/issues/42 --hostname gitlab.example.com"
	if got := strings.TrimSpace(string(log)); got != wantArgs {
		t.Fatalf("glab args = %q, want %q", got, wantArgs)
	}
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "issue",
		URL:       issueURL,
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	}); err != nil {
		t.Fatalf("create-issue live gate must accept the work_items alias: %v", err)
	}
}
