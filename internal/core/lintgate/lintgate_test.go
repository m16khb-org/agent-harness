package lintgate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintEditedGoFilesFlagsUnformatted(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}
	repo := t.TempDir()
	// Deliberately mis-indented (gofmt would reformat it).
	bad := filepath.Join(repo, "bad.go")
	if err := os.WriteFile(bad, []byte("package p\nfunc F()  {\nreturn\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	failed, feedback := LintEditedGoFiles(repo, []string{bad})
	if !failed {
		t.Fatalf("unformatted .go edit must be flagged, got failed=false feedback=%q", feedback)
	}
	if want := "bad.go"; !strings.Contains(feedback, want) {
		t.Fatalf("feedback must name the offending file %q, got %q", want, feedback)
	}
}

func TestLintEditedGoFilesCleanAndScope(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}
	repo := t.TempDir()
	good := filepath.Join(repo, "good.go")
	if err := os.WriteFile(good, []byte("package p\n\nfunc F() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if failed, fb := LintEditedGoFiles(repo, []string{good}); failed {
		t.Fatalf("formatted .go edit must not be flagged, got feedback=%q", fb)
	}
	// Non-Go paths are skipped entirely (no spawn, no flag).
	if failed, _ := LintEditedGoFiles(repo, []string{filepath.Join(repo, "README.md")}); failed {
		t.Fatal("non-Go edit must not be linted")
	}
	if failed, _ := LintEditedGoFiles(repo, nil); failed {
		t.Fatal("no paths must not be linted")
	}
}

// Fail-open: a bogus PATH (no gofmt) must NOT flag — a successful edit is never
// turned into a hook failure when the toolchain is absent.
func TestLintEditedGoFilesFailsOpenWithoutToolchain(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	repo := t.TempDir()
	bad := filepath.Join(repo, "bad.go")
	if err := os.WriteFile(bad, []byte("package p\nfunc F()  {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if failed, fb := LintEditedGoFiles(repo, []string{bad}); failed {
		t.Fatalf("missing toolchain must fail open (no flag), got feedback=%q", fb)
	}
}
