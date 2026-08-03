package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnoseNativeRuntimeDetectsCachedWorktreeExecutable(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	worktree := filepath.Join(base, "source.worktrees", "completed")
	gitdir := filepath.Join(source, ".git", "worktrees", "completed")
	mustMkdir(t, filepath.Join(source, ".git"))
	mustMkdir(t, worktree)
	mustMkdir(t, gitdir)
	mustWrite(t, filepath.Join(worktree, ".git"), "gitdir: "+gitdir+"\n")
	mustWrite(t, filepath.Join(gitdir, "commondir"), "../..\n")
	executable := filepath.Join(worktree, "bin", "agent-harness")
	mustMkdir(t, filepath.Dir(executable))
	mustWrite(t, executable, "old runtime")

	got, err := DiagnoseNativeRuntime(executable)
	if err != nil {
		t.Fatal(err)
	}
	wantExpected := filepath.Join(source, "bin", "agent-harness")
	if !got.Stale || got.Observed != executable || got.Expected != wantExpected || !got.RestartRequired {
		t.Fatalf("diagnostic = %+v, want stale observed=%q expected=%q restart", got, executable, wantExpected)
	}
}

func TestDiagnoseNativeRuntimeDetectsRemovedWorktreeByManagedLayout(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	mustMkdir(t, filepath.Join(source, ".git"))
	executable := filepath.Join(base, "source.worktrees", "completed", "bin", "agent-harness")
	mustMkdir(t, filepath.Dir(executable))
	mustWrite(t, executable, "unlinked runtime")

	got, err := DiagnoseNativeRuntime(executable)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Stale || got.Expected != filepath.Join(source, "bin", "agent-harness") {
		t.Fatalf("diagnostic = %+v, want managed worktree fallback", got)
	}
}

func TestDiagnoseNativeRuntimeAcceptsCanonicalSourceExecutable(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	mustMkdir(t, filepath.Join(source, ".git"))
	executable := filepath.Join(source, "bin", "agent-harness")
	mustMkdir(t, filepath.Dir(executable))
	mustWrite(t, executable, "current runtime")

	got, err := DiagnoseNativeRuntime(executable)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stale || got.Observed != executable || got.Expected != executable || got.RestartRequired {
		t.Fatalf("canonical diagnostic = %+v", got)
	}
}

func TestDiagnoseNativeRuntimeRejectsMalformedWorktreeMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source.worktrees", "broken")
	executable := filepath.Join(root, "bin", "agent-harness")
	mustMkdir(t, filepath.Dir(executable))
	mustWrite(t, executable, "broken runtime")
	mustWrite(t, filepath.Join(root, ".git"), "broken\n")

	_, err := DiagnoseNativeRuntime(executable)
	if err == nil || !strings.Contains(err.Error(), "gitdir") {
		t.Fatalf("error = %v, want gitdir diagnostic", err)
	}
	if _, statErr := os.Stat(executable); statErr != nil {
		t.Fatalf("fixture executable disappeared: %v", statErr)
	}
}
