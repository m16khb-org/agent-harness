package core

import (
	"strings"
	"testing"
)

func TestIssueOpsRemoteArtifactURLMatchesProvider(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "2-gitlab-mr"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://gitlab.example/group/project/-/issues/2"
	record.Phase = IssueOpsPhasePR
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://github.com/example/repo/pull/2",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "GitLab merge request URL") {
		t.Fatalf("gitlab remote artifact should reject GitHub PR URL, got %v", err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/not-a-number",
		Labels:    []string{"bug"},
		Assignees: []string{"100"},
	}); err == nil || !strings.Contains(err.Error(), "GitLab merge request URL") {
		t.Fatalf("gitlab remote artifact should reject nonnumeric MR URL, got %v", err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/other/project/-/merge_requests/2",
		Labels:    []string{"bug"},
		Assignees: []string{"100"},
	}); err == nil || !strings.Contains(err.Error(), "linked issue project") {
		t.Fatalf("gitlab remote artifact should reject MR URL from another project, got %v", err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/2",
		Labels:    []string{"bug"},
		Assignees: []string{"self"},
	}); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("gitlab remote artifact should reject placeholder assignee, got %v", err)
	}
	record, err = VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/2",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	})
	if err != nil {
		t.Fatalf("gitlab remote artifact should accept GitLab MR URL: %v", err)
	}
	if record.RemoteArtifact == nil || record.RemoteArtifact.Provider != "gitlab" || record.RemoteArtifact.Kind != "mr" {
		t.Fatalf("unexpected gitlab remote artifact: %+v", record.RemoteArtifact)
	}
}

func TestIssueOpsChildLinksPersistProviderNeutralGraph(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/10")
	if err != nil {
		t.Fatal(err)
	}

	record, err = LinkIssueOpsChild(stateRoot, record.ID, "https://github.com/example/repo/issues/11", "write child graph tests")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.IssueLinks) != 1 {
		t.Fatalf("expected one child issue link, got %+v", record.IssueLinks)
	}
	link := record.IssueLinks[0]
	if link.Type != "child" || link.URL != "https://github.com/example/repo/issues/11" || link.Title != "write child graph tests" || link.Provider != "github" {
		t.Fatalf("unexpected child issue link: %+v", link)
	}
	if link.CreatedAt == "" {
		t.Fatalf("child issue link should record created_at: %+v", link)
	}

	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.IssueLinks) != 1 || reloaded.IssueLinks[0].URL != link.URL {
		t.Fatalf("reloaded child issue links mismatch: %+v", reloaded.IssueLinks)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, link.URL, "duplicate"); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("expected duplicate child link rejection, got %v", err)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child"); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("generic child under GitHub parent should be rejected as provider mismatch, got %v", err)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, "https://github.com/other/repo/issues/12", "other repo child"); err == nil || !strings.Contains(err.Error(), "parent issue project") {
		t.Fatalf("GitHub child from another repo should be rejected, got %v", err)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, "https://github.com/example/repo/issues/not-a-number", "bad child"); err == nil || !strings.Contains(err.Error(), "numeric github issue URL") {
		t.Fatalf("GitHub child with nonnumeric issue should be rejected, got %v", err)
	}
	gitlab, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/gitlab", Branch: "20-gitlab"})
	if err != nil {
		t.Fatal(err)
	}
	gitlab, err = LinkIssueOpsIssue(stateRoot, gitlab.ID, "https://gitlab.example/group/project/-/issues/20")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsChild(stateRoot, gitlab.ID, "https://gitlab.example/other/project/-/issues/21", "other project child"); err == nil || !strings.Contains(err.Error(), "parent issue project") {
		t.Fatalf("GitLab child from another project should be rejected, got %v", err)
	}
	if _, err := LinkIssueOpsChild(stateRoot, gitlab.ID, "https://gitlab.example/group/project/-/issues/not-a-number", "bad child"); err == nil || !strings.Contains(err.Error(), "numeric gitlab issue URL") {
		t.Fatalf("GitLab child with nonnumeric issue should be rejected, got %v", err)
	}
	gitlab, err = LinkIssueOpsChild(stateRoot, gitlab.ID, "https://gitlab.example/group/project/-/issues/21", "same project child")
	if err != nil {
		t.Fatalf("GitLab child in same project should be accepted: %v", err)
	}
	generic, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/generic", Branch: "10-generic"})
	if err != nil {
		t.Fatal(err)
	}
	generic, err = LinkIssueOpsIssue(stateRoot, generic.ID, "https://tracker.example/acme/repo/issues/10")
	if err != nil {
		t.Fatal(err)
	}
	generic, err = LinkIssueOpsChild(stateRoot, generic.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child")
	if err != nil {
		t.Fatal(err)
	}
	if got := generic.IssueLinks[0].Provider; got != "" {
		t.Fatalf("generic issue URL should not infer a provider, got %q", got)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, "not-a-url", "bad"); err == nil || !strings.Contains(err.Error(), "child_url") {
		t.Fatalf("expected child URL validation error, got %v", err)
	}
}
