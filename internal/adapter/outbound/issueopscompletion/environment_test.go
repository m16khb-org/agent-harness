package issueopscompletion

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Environment outbound 어댑터의 경로/HEAD 검증 래퍼를 잠근다.
func TestEnvironmentPathsMatchResolvesAndTrims(t *testing.T) {
	env := NewEnvironment()
	dir := t.TempDir()
	trimmed := "  " + dir + "  "
	if !env.PathsMatch(dir, trimmed) {
		t.Fatal("trimmed and resolved identical paths must match")
	}
	if env.PathsMatch(dir, filepath.Join(t.TempDir(), "other")) {
		t.Fatal("distinct paths must not match")
	}
}

func TestEnvironmentCurrentHeadRunsGit(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@e.invalid")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "f")
	run("commit", "-q", "-m", "init")
	want := strings.TrimSpace(mustCombinedOutput(t, repo, "rev-parse", "HEAD"))
	got, err := NewEnvironment().CurrentHead(context.Background(), repo)
	if err != nil || got != want || got == "" {
		t.Fatalf("CurrentHead = %q err=%v want=%q", got, err, want)
	}
	if _, err := NewEnvironment().CurrentHead(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing repo must fail")
	}
}

func mustCombinedOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return string(out)
}
