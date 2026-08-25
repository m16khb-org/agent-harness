package augmentplan

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHasImplementationDeltaDetectsUncommittedWork(t *testing.T) {
	t.Run("directory without git reports false", func(t *testing.T) {
		if hasImplementationDelta(t.TempDir()) {
			t.Fatal("directory without git must report no implementation delta")
		}
	})

	t.Run("clean repository reports false", func(t *testing.T) {
		root := initGitRepoForEvidenceTest(t)
		if hasImplementationDelta(root) {
			t.Fatal("clean repository must report no implementation delta")
		}
	})

	t.Run("dirty repository reports true", func(t *testing.T) {
		root := initGitRepoForEvidenceTest(t)
		if err := os.WriteFile(filepath.Join(root, "change.go"), []byte("package x\n"), 0o644); err != nil {
			t.Fatalf("write change file failed: %v", err)
		}
		if !hasImplementationDelta(root) {
			t.Fatal("uncommitted file must count as implementation delta")
		}
	})
}

func initGitRepoForEvidenceTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}
	return root
}

func TestGoalEvidenceAlwaysNamesObservable(t *testing.T) {
	if got := implementationEvidence(t.TempDir()); len(got) == 0 || got[0] == "" {
		t.Fatal("implementation evidence must always name an observable")
	}
	if got := verificationEvidence(); len(got) == 0 || got[0] == "" {
		t.Fatal("verification evidence must always name an observable")
	}
	if got := learningEvidence(); len(got) == 0 || got[0] == "" {
		t.Fatal("learning evidence must always name an observable")
	}
}
