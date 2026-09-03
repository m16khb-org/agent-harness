package gitlab

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
}

func TestGitLabArtifactEndpointResolvesKinds(t *testing.T) {
	tests := []struct {
		name, kind, url, wantHost, wantEndpoint string
		wantErr                                 string
	}{
		{
			name: "issue", kind: "issue", url: "https://gitlab.example.com/acme/repo/-/issues/12",
			wantHost: "gitlab.example.com", wantEndpoint: "projects/acme%2Frepo/issues/12",
		},
		{
			// 사내 GitLab은 이슈 web_url을 /-/work_items/로 돌려준다. 같은 identity다.
			name: "child work item", kind: "child", url: "https://gitlab.example.com/acme/repo/-/work_items/31",
			wantHost: "gitlab.example.com", wantEndpoint: "projects/acme%2Frepo/issues/31",
		},
		{
			name: "merge request", kind: "mr", url: "https://gitlab.example.com/acme/repo/-/merge_requests/9",
			wantHost: "gitlab.example.com", wantEndpoint: "projects/acme%2Frepo/merge_requests/9",
		},
		{name: "github kind", kind: "pr", url: "https://gitlab.example.com/acme/repo/-/merge_requests/9", wantErr: "unsupported gitlab artifact kind"},
		{name: "mismatched url", kind: "mr", url: "https://gitlab.example.com/acme/repo/-/issues/12", wantErr: "merge request URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, endpoint, err := glabArtifactEndpoint(tt.kind, tt.url)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if host != tt.wantHost || endpoint != tt.wantEndpoint {
				t.Fatalf("host=%q endpoint=%q, want %q / %q", host, endpoint, tt.wantHost, tt.wantEndpoint)
			}
		})
	}
}

func TestGitLabReadArtifactBodyReturnsDescriptionAndState(t *testing.T) {
	skipOnWindows(t)
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '{"description":"본문","state":"opened","web_url":"https://gitlab.example.com/acme/repo/-/merge_requests/9"}'
exit 0
`)
	t.Setenv("PATH", binDir)
	got, err := NewProvider().ReadArtifactBody(context.Background(), port.IssueProviderArtifactBodyRequest{
		Repo: repo, Kind: "mr", URL: "https://gitlab.example.com/acme/repo/-/merge_requests/9",
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Body != "본문" || got.State != "opened" {
		t.Fatalf("body=%q state=%q", got.Body, got.State)
	}
}

func TestGitLabReplaceArtifactBodyPreviewDoesNotExecute(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir) // no glab on PATH: a preview must not need one
	res, err := NewProvider().ReplaceArtifactBody(context.Background(), port.IssueProviderReplaceArtifactBodyRequest{
		Repo: t.TempDir(), Kind: "issue", URL: "https://gitlab.example.com/acme/repo/-/issues/12", Body: "새 본문",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if res.Updated {
		t.Fatal("a preview must not report an update")
	}
	if !strings.Contains(res.Preview, "--method PUT") || !strings.Contains(res.Preview, "projects/acme%2Frepo/issues/12") {
		t.Fatalf("preview = %q", res.Preview)
	}
}

func TestGitLabReplaceArtifactBodyWritesAndVerifies(t *testing.T) {
	skipOnWindows(t)
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
case "$*" in
  *"--method PUT"*)
    printf '%s' "$*" > glab.put
    printf '{"web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'
    exit 0
    ;;
esac
if [ -f glab.put ]; then
  printf '{"description":"새 본문","state":"opened","web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'
else
  printf '{"description":"옛 본문","state":"opened","web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'
fi
exit 0
`)
	t.Setenv("PATH", binDir)
	res, err := NewProvider().ReplaceArtifactBody(context.Background(), port.IssueProviderReplaceArtifactBodyRequest{
		Repo: repo, Kind: "issue", URL: "https://gitlab.example.com/acme/repo/-/issues/12", Body: "새 본문", Confirm: true,
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !res.Updated || res.VerifiedBodySHA256 == "" {
		t.Fatalf("result = %+v", res)
	}
	put, err := os.ReadFile(filepath.Join(repo, "glab.put"))
	if err != nil || !strings.Contains(string(put), "description=새 본문") {
		t.Fatalf("PUT argv = %q err=%v", put, err)
	}
}

func TestGitLabReplaceArtifactBodyFailsWhenReadbackDiffers(t *testing.T) {
	skipOnWindows(t)
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
case "$*" in
  *"--method PUT"*) printf '{"web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'; exit 0 ;;
esac
printf '{"description":"다른 본문","state":"opened","web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'
exit 0
`)
	t.Setenv("PATH", binDir)
	_, err := NewProvider().ReplaceArtifactBody(context.Background(), port.IssueProviderReplaceArtifactBodyRequest{
		Repo: repo, Kind: "issue", URL: "https://gitlab.example.com/acme/repo/-/issues/12", Body: "새 본문", Confirm: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("a divergent readback must fail the write, got %v", err)
	}
}
