package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestVerifyIssueOpsRemoteArtifactLiveRejectsMissingGitHubPR(t *testing.T) {
	installFakeGHForRemoteArtifactTest(t)
	err := verifyIssueOpsRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/9999",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing GitHub PR to fail live verification, got %v", err)
	}
}

func TestVerifyIssueOpsRemoteArtifactLiveRequiresRemoteLabelsAndAssignees(t *testing.T) {
	installFakeGHForRemoteArtifactTest(t)
	if err := verifyIssueOpsRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/1",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}); err != nil {
		t.Fatalf("expected matching GitHub PR evidence to pass: %v", err)
	}
	if err := verifyIssueOpsRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/1",
		Labels:    []string{"missing"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("expected missing label to fail live verification, got %v", err)
	}
	if err := verifyIssueOpsRemoteArtifactLive(core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/1",
		Labels:    []string{"bug"},
		Assignees: []string{"other"},
	}); err == nil || !strings.Contains(err.Error(), "assignee") {
		t.Fatalf("expected missing assignee to fail live verification, got %v", err)
	}
}

func TestVerifyIssueOpsRemoteArtifactMergedLiveRequiresMergedGitHubPR(t *testing.T) {
	installFakeGHForRemoteArtifactTest(t)
	if err := verifyIssueOpsRemoteArtifactMergedLive(core.IssueOpsRemoteArtifactVerification{
		Provider: "github",
		Kind:     "pr",
		URL:      "https://github.com/example/repo/pull/2",
	}); err == nil || !strings.Contains(err.Error(), "not verified merged") {
		t.Fatalf("expected closed unmerged GitHub PR to fail cleanup merge verification, got %v", err)
	}
	if err := verifyIssueOpsRemoteArtifactMergedLive(core.IssueOpsRemoteArtifactVerification{
		Provider: "github",
		Kind:     "pr",
		URL:      "https://github.com/example/repo/pull/3",
	}); err != nil {
		t.Fatalf("expected merged GitHub PR to pass cleanup merge verification: %v", err)
	}
}

func TestSplitGitLabMRPath_whenValidEscapedPath(t *testing.T) {
	cases := []struct {
		name        string
		escapedPath string
		wantProject string
		wantIID     string
	}{
		{
			name:        "nested project",
			escapedPath: "/group/subgroup/project/-/merge_requests/42",
			wantProject: "group/subgroup/project",
			wantIID:     "42",
		},
		{
			name:        "escaped project segment",
			escapedPath: "/platform/team%20space/repo/-/merge_requests/7",
			wantProject: "platform/team space/repo",
			wantIID:     "7",
		},
		{
			name:        "unclean path",
			escapedPath: "group/project/../project/-/merge_requests/9/",
			wantProject: "group/project",
			wantIID:     "9",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts := splitGitLabMRPath(tc.escapedPath)
			if parts.project != tc.wantProject || parts.iid != tc.wantIID {
				t.Fatalf("splitGitLabMRPath(%q) = %+v, want project=%q iid=%q", tc.escapedPath, parts, tc.wantProject, tc.wantIID)
			}
		})
	}
}

func TestSplitGitLabMRPath_whenInvalidPath(t *testing.T) {
	cases := []struct {
		escapedPath string
		wantIID     string
	}{
		{escapedPath: "/group/project/-/issues/42"},
		{escapedPath: "/group/project/-/merge_requests/not-number"},
		{escapedPath: "/-/merge_requests/42", wantIID: "42"},
		{escapedPath: "/group/project/merge_requests/42"},
	}
	for _, tc := range cases {
		t.Run(tc.escapedPath, func(t *testing.T) {
			if parts := splitGitLabMRPath(tc.escapedPath); parts.project != "" || parts.iid != tc.wantIID {
				t.Fatalf("splitGitLabMRPath(%q) = %+v, want project empty iid=%q", tc.escapedPath, parts, tc.wantIID)
			}
		})
	}
}

func TestSplitGitLabIssuePath_whenValidEscapedPath(t *testing.T) {
	cases := []struct {
		name        string
		escapedPath string
		wantProject string
		wantIID     string
	}{
		{
			name:        "nested project",
			escapedPath: "/group/subgroup/project/-/issues/42",
			wantProject: "group/subgroup/project",
			wantIID:     "42",
		},
		{
			name:        "escaped project segment",
			escapedPath: "/platform/team%20space/repo/-/issues/7",
			wantProject: "platform/team space/repo",
			wantIID:     "7",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts := splitGitLabIssuePath(tc.escapedPath)
			if parts.project != tc.wantProject || parts.iid != tc.wantIID {
				t.Fatalf("splitGitLabIssuePath(%q) = %+v, want project=%q iid=%q", tc.escapedPath, parts, tc.wantProject, tc.wantIID)
			}
		})
	}
}

func TestSplitGitLabIssuePath_whenInvalidPath(t *testing.T) {
	cases := []struct {
		escapedPath string
		wantIID     string
	}{
		{escapedPath: "/group/project/-/merge_requests/42"},
		{escapedPath: "/group/project/-/issues/not-number"},
		{escapedPath: "/-/issues/42", wantIID: "42"},
		{escapedPath: "/group/project/issues/42"},
	}
	for _, tc := range cases {
		t.Run(tc.escapedPath, func(t *testing.T) {
			if parts := splitGitLabIssuePath(tc.escapedPath); parts.project != "" || parts.iid != tc.wantIID {
				t.Fatalf("splitGitLabIssuePath(%q) = %+v, want project empty iid=%q", tc.escapedPath, parts, tc.wantIID)
			}
		})
	}
}

func installFakeGHForRemoteArtifactTest(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	gh := filepath.Join(bin, "gh")
	script := `#!/bin/sh
case "$3" in
  *"/pull/1")
    printf '%s\n' '{"url":"https://github.com/example/repo/pull/1","labels":[{"name":"bug"}],"assignees":[{"login":"habin","name":"Habin"}],"state":"OPEN"}'
    exit 0
    ;;
  *"/pull/2")
    printf '%s\n' '{"url":"https://github.com/example/repo/pull/2","labels":[{"name":"bug"}],"assignees":[{"login":"habin","name":"Habin"}],"state":"CLOSED","mergedAt":null}'
    exit 0
    ;;
  *"/pull/3")
    printf '%s\n' '{"url":"https://github.com/example/repo/pull/3","labels":[{"name":"bug"}],"assignees":[{"login":"habin","name":"Habin"}],"state":"MERGED","mergedAt":"2026-06-05T11:00:00Z"}'
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
