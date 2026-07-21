package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/core/preflight"
)

func TestGitIssueOpsHandoffPublicationReaderPushExactSetsBranchUpstream(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := initIssueOpsRepo(t)
	for _, args := range [][]string{
		{"checkout", "-q", "-b", "feature"},
		{"commit", "--allow-empty", "-q", "-m", "feature"},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	code, stdout, stderr := preflight.GitCmd(repo, "rev-parse", "HEAD")
	if code != 0 {
		t.Fatalf("git rev-parse HEAD failed: %s", stderr)
	}
	reader := GitIssueOpsHandoffPublicationReader{}
	target, err := reader.PushTarget(context.Background(), repo, "origin")
	if err != nil {
		t.Fatalf("resolve push target: %v", err)
	}
	if err := reader.PushExact(context.Background(), repo, "origin", target.Fingerprint, "feature", strings.TrimSpace(stdout)); err != nil {
		t.Fatalf("push exact: %v", err)
	}
	code, stdout, stderr = preflight.GitCmd(repo, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "feature@{upstream}")
	if code != 0 {
		t.Fatalf("git rev-parse feature upstream failed: %s", stderr)
	}
	if got, want := strings.TrimSpace(stdout), "origin/feature"; got != want {
		t.Fatalf("feature upstream = %q, want %q", got, want)
	}
}

func TestGitIssueOpsHandoffPublicationReaderSerializesConcurrentPushExact(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := initIssueOpsRepo(t)
	for _, args := range [][]string{
		{"checkout", "-q", "-b", "feature"},
		{"commit", "--allow-empty", "-q", "-m", "feature"},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	code, stdout, stderr := preflight.GitCmd(repo, "rev-parse", "HEAD")
	if code != 0 {
		t.Fatalf("git rev-parse HEAD failed: %s", stderr)
	}
	reader := GitIssueOpsHandoffPublicationReader{}
	target, err := reader.PushTarget(context.Background(), repo, "origin")
	if err != nil {
		t.Fatalf("resolve push target: %v", err)
	}

	const callers = 4
	start := make(chan struct{})
	errs := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			errs <- reader.PushExact(context.Background(), repo, "origin", target.Fingerprint, "feature", strings.TrimSpace(stdout))
		}()
	}
	close(start)
	for range callers {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent push exact: %v", err)
		}
	}
}
