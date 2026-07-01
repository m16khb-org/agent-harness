package remoteverify

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"agent-harness/internal/core"
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
    printf '%s\n' '{"url":"https://github.com/example/repo/issues/1","labels":[{"name":"bug"}],"assignees":[{"login":"habin","name":"Habin"}],"state":"OPEN"}'
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
	err := VerifyRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "issue",
		URL:       "https://github.com/example/repo/issues/9999",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing GitHub issue to fail live verification, got %v", err)
	}
}

func TestVerifyIssueOpsRemoteArtifactLiveRequiresGitHubIssueLabelsAndAssignees(t *testing.T) {
	installFakeGHIssueForRemoteArtifactTest(t)
	if err := VerifyRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "issue",
		URL:       "https://github.com/example/repo/issues/1",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}); err != nil {
		t.Fatalf("expected matching GitHub issue evidence to pass: %v", err)
	}
	if err := VerifyRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "issue",
		URL:       "https://github.com/example/repo/issues/1",
		Labels:    []string{"missing"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("expected missing label to fail live verification, got %v", err)
	}
	if err := VerifyRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
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
printf '%s\n' "$*" > "$HARNESS_FAKE_GLAB_LOG"
printf '%s\n' '{"web_url":"https://gitlab.example.com/group/project/-/issues/42","state":"opened","labels":["ready","bug"],"assignees":[{"id":123,"username":"habin","name":"Ha Bin"},{"id":0,"username":"reviewer","name":"Reviewer"}]}'
`)
	t.Setenv("HARNESS_FAKE_GLAB_LOG", logPath)
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
	for _, want := range []string{"123", "habin", "Ha Bin", "reviewer", "Reviewer"} {
		if !slices.Contains(artifact.Assignees, want) {
			t.Fatalf("missing assignee %q in %#v", want, artifact.Assignees)
		}
	}
}

func TestVerifyIssueOpsRemoteArtifactLiveRequiresGitLabIssueLabelsAndAssignees(t *testing.T) {
	bin := t.TempDir()
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' '{"web_url":"https://gitlab.example.com/group/project/-/issues/42","state":"opened","labels":["bug"],"assignees":[{"id":1,"username":"habin","name":"Habin"}]}'
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := VerifyRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "issue",
		URL:       "https://gitlab.example.com/group/project/-/issues/42",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}); err != nil {
		t.Fatalf("expected matching GitLab issue evidence to pass: %v", err)
	}
	if err := VerifyRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "issue",
		URL:       "https://gitlab.example.com/group/project/-/issues/42",
		Labels:    []string{"missing"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("expected missing label to fail live verification, got %v", err)
	}
}

func TestFetchGitLabIssueArtifactRejectsNonIssueURL(t *testing.T) {
	_, err := fetchGitLabIssueArtifact("https://gitlab.example.com/group/project/-/merge_requests/42")
	if err == nil || !strings.Contains(err.Error(), "remote artifact url must be a GitLab issue URL") {
		t.Fatalf("expected invalid issue URL error, got %v", err)
	}
	if _, err := fetchGitLabIssueArtifact("https://gitlab.example.com/group/project/-/work_items/42"); err == nil {
		t.Fatal("work item URL must not parse as a GitLab issue artifact")
	}
}
