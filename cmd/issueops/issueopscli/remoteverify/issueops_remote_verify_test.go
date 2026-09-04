package remoteverify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestVerifyRemoteArtifactLiveRejectsUnsupportedCombination(t *testing.T) {
	err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider: "other",
		Kind:     "issue",
		URL:      "https://example.com/acme/repo/issues/1",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v, want unsupported verifier error", err)
	}
}

func TestVerifyRemoteArtifactLiveContextHonorsCancellation(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "invoked")
	writeFakeCommand(t, filepath.Join(bin, "gh"), "#!/bin/sh\ntouch \"$VERIFY_MARKER\"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("VERIFY_MARKER", marker)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := VerifyRemoteArtifactLiveContext(ctx, issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider: "github",
		Kind:     "issue",
		URL:      "https://github.com/acme/repo/issues/1",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("verification command ran after cancellation: %v", statErr)
	}
}

func TestCommandOutputErrorRedactsAndCapsProviderDiagnostics(t *testing.T) {
	raw := "token=super-secret https://internal.example/path " + strings.Repeat("x", 4096)

	err := commandOutputError(&exec.ExitError{Stderr: []byte(raw)})

	if strings.Contains(err.Error(), "super-secret") || strings.Contains(err.Error(), "internal.example") {
		t.Fatalf("diagnostic was not redacted: %q", err)
	}
	if len(err.Error()) > maxRemoteVerifyDiagnosticBytes {
		t.Fatalf("diagnostic length = %d, want <= %d", len(err.Error()), maxRemoteVerifyDiagnosticBytes)
	}
}

func TestVerifyIssueOpsRemoteArtifactLiveRejectsMissingGitHubPR(t *testing.T) {
	installFakeGHForRemoteArtifactTest(t)
	err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/9999",
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing GitHub PR to fail live verification, got %v", err)
	}
}

func TestVerifyIssueOpsRemoteArtifactLiveRequiresRemoteLabelsAndAssignees(t *testing.T) {
	installFakeGHForRemoteArtifactTest(t)
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/1",
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	}); err != nil {
		t.Fatalf("expected matching GitHub PR evidence to pass: %v", err)
	}
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/1",
		Labels:    []string{"missing"},
		Assignees: []string{"sample"},
	}); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("expected missing label to fail live verification, got %v", err)
	}
	if err := VerifyRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
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
	if err := VerifyRemoteArtifactMergedLive(issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "github",
		Kind:     "pr",
		URL:      "https://github.com/example/repo/pull/2",
	}); err == nil || !strings.Contains(err.Error(), "not verified merged") {
		t.Fatalf("expected closed unmerged GitHub PR to fail cleanup merge verification, got %v", err)
	}
	if err := VerifyRemoteArtifactMergedLive(issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "github",
		Kind:     "pr",
		URL:      "https://github.com/example/repo/pull/3",
	}); err != nil {
		t.Fatalf("expected merged GitHub PR to pass cleanup merge verification: %v", err)
	}
}

func TestFetchGitLabMergeRequestArtifactParsesLivePayload(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "glab.log")
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' "$*" > "$ISSUEOPS_FAKE_GLAB_LOG"
printf '%s\n' '{"web_url":"https://gitlab.example.com/group/project/-/merge_requests/42","state":"merged","merged_at":"","labels":["ready","refactor"],"assignees":[{"id":123,"username":"sample","name":"Ha Bin"},{"id":0,"username":"reviewer","name":"Reviewer"}]}'
`)
	t.Setenv("ISSUEOPS_FAKE_GLAB_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	artifact, err := fetchGitLabMergeRequestArtifact("https://gitlab.example.com/group/project/-/merge_requests/42")
	if err != nil {
		t.Fatalf("fetch GitLab MR artifact: %v", err)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := "api projects/group%2Fproject/merge_requests/42 --hostname gitlab.example.com"
	if got := strings.TrimSpace(string(log)); got != wantArgs {
		t.Fatalf("glab args = %q, want %q", got, wantArgs)
	}
	if artifact.URL != "https://gitlab.example.com/group/project/-/merge_requests/42" || !artifact.Merged {
		t.Fatalf("unexpected artifact identity: %#v", artifact)
	}
	for _, want := range []string{"ready", "refactor"} {
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

// #116 게이트 ⑨·⑩은 머지 검증과 같은 readback에서 head ref 정체를 함께
// 받아야 한다. GitHub은 gh --json에 두 필드를 추가하고, GitLab은 기제공되는
// source_branch·sha를 매핑한다.
func TestVerifyRemoteArtifactMergedHeadLiveReturnsGitHubHeadRef(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "gh.log")
	writeFakeCommand(t, filepath.Join(bin, "gh"), `#!/bin/sh
printf '%s\n' "$*" > "$ISSUEOPS_FAKE_GH_LOG"
printf '%s\n' '{"url":"https://github.com/example/repo/pull/3","state":"MERGED","mergedAt":"2026-06-05T11:00:00Z","headRefName":"116-remote-branch-delete","headRefOid":"1111111111111111111111111111111111111111","baseRefName":"main","labels":[],"assignees":[]}'
`)
	t.Setenv("ISSUEOPS_FAKE_GH_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	head, err := VerifyRemoteArtifactMergedHeadLive(issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/repo/pull/3",
	})
	if err != nil {
		t.Fatalf("merged head verification must pass: %v", err)
	}
	if head.HeadRefName != "116-remote-branch-delete" || head.HeadRefOID != "1111111111111111111111111111111111111111" {
		t.Fatalf("unexpected head projection: %#v", head)
	}
	// cleanup finish의 base drift 게이트는 머지 관측과 같은 시점의 base를 요구한다.
	if head.BaseRefName != "main" {
		t.Fatalf("merged readback must carry the observed base ref: %#v", head)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"headRefName", "headRefOid", "baseRefName"} {
		if !strings.Contains(string(log), want) {
			t.Fatalf("gh --json field list must request %q: %q", want, strings.TrimSpace(string(log)))
		}
	}
}

func TestVerifyRemoteArtifactMergedHeadLiveReturnsGitLabSourceBranch(t *testing.T) {
	bin := t.TempDir()
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' '{"web_url":"https://gitlab.example.com/group/project/-/merge_requests/42","state":"merged","merged_at":"","source_branch":"116-remote-branch-delete","target_branch":"main","sha":"2222222222222222222222222222222222222222","labels":[],"assignees":[]}'
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	head, err := VerifyRemoteArtifactMergedHeadLive(issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "gitlab", Kind: "mr", URL: "https://gitlab.example.com/group/project/-/merge_requests/42",
	})
	if err != nil {
		t.Fatalf("merged head verification must pass: %v", err)
	}
	if head.HeadRefName != "116-remote-branch-delete" || head.HeadRefOID != "2222222222222222222222222222222222222222" {
		t.Fatalf("unexpected head projection: %#v", head)
	}
	if head.BaseRefName != "main" {
		t.Fatalf("merged readback must carry the observed target branch: %#v", head)
	}
}

func TestVerifyRemoteArtifactMergedHeadLiveRejectsUnmerged(t *testing.T) {
	installFakeGHForRemoteArtifactTest(t)
	if _, err := VerifyRemoteArtifactMergedHeadLive(issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/repo/pull/2",
	}); err == nil || !strings.Contains(err.Error(), "not verified merged") {
		t.Fatalf("unmerged PR must fail head verification: %v", err)
	}
}

func TestFetchGitLabMergeRequestArtifactRejectsInvalidMRURL(t *testing.T) {
	_, err := fetchGitLabMergeRequestArtifact("https://gitlab.example.com/group/project/-/issues/42")
	if err == nil || !strings.Contains(err.Error(), "remote artifact url must be a GitLab merge request URL") {
		t.Fatalf("expected invalid MR URL error, got %v", err)
	}
}

func TestFetchGitLabMergeRequestArtifactReportsCLIAndDecodeErrors(t *testing.T) {
	t.Run("cli stderr", func(t *testing.T) {
		bin := t.TempDir()
		writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
echo "merge request not found" >&2
exit 1
`)
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

		_, err := fetchGitLabMergeRequestArtifact("https://gitlab.example.com/group/project/-/merge_requests/404")
		if err == nil || !strings.Contains(err.Error(), "verify GitLab MR through glab failed: merge request not found") {
			t.Fatalf("expected glab stderr in error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		bin := t.TempDir()
		writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' '{invalid'
`)
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

		_, err := fetchGitLabMergeRequestArtifact("https://gitlab.example.com/group/project/-/merge_requests/42")
		if err == nil || !strings.Contains(err.Error(), "decode GitLab MR verification") {
			t.Fatalf("expected decode error, got %v", err)
		}
	})
}
