package github

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
)

// PATH is pinned to the stub directory in these tests, so the fake gh may use
// shell builtins only — no cat, no sed.
const ghBodyStub = `#!/bin/sh
printf '%s\n' "$@" > "gh.$1.$2.argv"
if [ "$2" = "edit" ]; then
  body=""
  while IFS= read -r line || [ -n "$line" ]; do body="$body$line"; done
  printf '%s' "$body" > gh.body
  exit 0
fi
if [ "$2" = "view" ]; then
  saved="old body"
  if [ -f gh.body ]; then IFS= read -r saved < gh.body; fi
  printf '{"body":"%s","state":"OPEN"}' "$saved"
  exit 0
fi
exit 2
`

func TestGitHubReadArtifactBodyRejectsForeignKind(t *testing.T) {
	_, err := NewProvider().ReadArtifactBody(context.Background(), port.IssueProviderArtifactBodyRequest{
		Kind: "mr", URL: "https://github.com/acme/repo/issues/1",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported github artifact kind") {
		t.Fatalf("mr is GitLab's kind and must be refused, got %v", err)
	}
}

func TestGitHubReadArtifactBodyUsesTheKindNoun(t *testing.T) {
	for _, tt := range []struct{ kind, noun string }{
		{"issue", "issue"}, {"child", "issue"}, {"pr", "pr"},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			binDir, repo := t.TempDir(), t.TempDir()
			writeFakeGh(t, binDir, ghBodyStub)
			t.Setenv("PATH", binDir)
			got, err := NewProvider().ReadArtifactBody(context.Background(), port.IssueProviderArtifactBodyRequest{
				Repo: repo, Kind: tt.kind, URL: "https://github.com/acme/repo/issues/7",
			})
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if got.Body != "old body" || got.State != "OPEN" {
				t.Fatalf("read body=%q state=%q", got.Body, got.State)
			}
			if _, err := os.Stat(filepath.Join(repo, "gh."+tt.noun+".view.argv")); err != nil {
				t.Fatalf("kind %q must address the %q noun: %v", tt.kind, tt.noun, err)
			}
		})
	}
}

func TestGitHubReplaceArtifactBodyPreviewDoesNotExecute(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir) // no gh on PATH: a preview must not need one
	res, err := NewProvider().ReplaceArtifactBody(context.Background(), port.IssueProviderReplaceArtifactBodyRequest{
		Repo: t.TempDir(), Kind: "issue", URL: "https://github.com/acme/repo/issues/7", Body: "new body",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if res.Updated {
		t.Fatal("a preview must not report an update")
	}
	if !strings.HasPrefix(res.Preview, "[dry-run]") || !strings.Contains(res.Preview, "--body-file") {
		t.Fatalf("preview = %q", res.Preview)
	}
}

func TestGitHubReplaceArtifactBodyWritesOnStdinAndVerifies(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, binDir, ghBodyStub)
	t.Setenv("PATH", binDir)
	res, err := NewProvider().ReplaceArtifactBody(context.Background(), port.IssueProviderReplaceArtifactBodyRequest{
		Repo: repo, Kind: "pr", URL: "https://github.com/acme/repo/pull/9", Body: "new body", Confirm: true,
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !res.Updated || res.VerifiedBodySHA256 == "" {
		t.Fatalf("result = %+v", res)
	}
	argv, err := os.ReadFile(filepath.Join(repo, "gh.pr.edit.argv"))
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if want := "pr\nedit\nhttps://github.com/acme/repo/pull/9\n--body-file\n-\n"; string(argv) != want {
		t.Fatalf("edit argv = %q, want %q", argv, want)
	}
	written, err := os.ReadFile(filepath.Join(repo, "gh.body"))
	if err != nil || string(written) != "new body" {
		t.Fatalf("body must arrive on stdin, got %q err=%v", written, err)
	}
}

func TestGitHubReplaceArtifactBodyFailsWhenReadbackDiffers(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$2" = "edit" ]; then cat > /dev/null; exit 0; fi
if [ "$2" = "view" ]; then printf '{"body":"something else","state":"OPEN"}'; exit 0; fi
exit 2
`)
	t.Setenv("PATH", binDir)
	_, err := NewProvider().ReplaceArtifactBody(context.Background(), port.IssueProviderReplaceArtifactBodyRequest{
		Repo: repo, Kind: "issue", URL: "https://github.com/acme/repo/issues/7", Body: "new body", Confirm: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("a divergent readback must fail the write, got %v", err)
	}
}

func TestGitHubVerifyChildHierarchy(t *testing.T) {
	for _, tt := range []struct {
		name  string
		child string
		want  bool
	}{
		{"attached", "https://github.com/acme/repo/issues/31", true},
		{"unrelated", "https://github.com/acme/repo/issues/99", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			binDir, repo := t.TempDir(), t.TempDir()
			writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1" = "api" ]; then printf '[{"id":501,"number":31}]'; exit 0; fi
exit 2
`)
			t.Setenv("PATH", binDir)
			got, err := NewProvider().VerifyChildHierarchy(context.Background(), port.IssueProviderChildHierarchyRequest{
				Repo: repo, ParentIssueURL: "https://github.com/acme/repo/issues/12", ChildURL: tt.child,
			})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if got.Verified != tt.want {
				t.Fatalf("verified = %v, want %v", got.Verified, tt.want)
			}
		})
	}
}
