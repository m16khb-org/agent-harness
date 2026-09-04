package goformat

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReportsUnformattedTrackedFiles(t *testing.T) {
	deps := Deps{
		ListTrackedGoFiles: func(context.Context, string) ([]string, error) {
			return []string{"cmd/a.go", "internal/b.go"}, nil
		},
		ListUnformatted: func(_ context.Context, _ string, files []string) ([]string, error) {
			if len(files) != 2 {
				t.Fatalf("gofmt must receive every tracked file, got %v", files)
			}
			return []string{"internal/b.go"}, nil
		},
	}

	step := ValidateWithDeps("/repo", deps)

	if step.OK || step.Label != Label || step.Command != Command {
		t.Fatalf("unexpected step: %+v", step)
	}
	if !strings.Contains(step.Error, "internal/b.go") || strings.Contains(step.Error, "cmd/a.go") || !strings.Contains(step.Error, "gofmt -w") {
		t.Fatalf("error must name only the unformatted file and the fix: %q", step.Error)
	}
}

func TestValidatePassesWhenEveryTrackedFileIsFormatted(t *testing.T) {
	deps := Deps{
		ListTrackedGoFiles: func(context.Context, string) ([]string, error) { return []string{"cmd/a.go", "internal/b.go"}, nil },
		ListUnformatted:    func(context.Context, string, []string) ([]string, error) { return nil, nil },
	}

	step := ValidateWithDeps("/repo", deps)

	if !step.OK || step.Error != "" || !strings.Contains(step.Stdout, "checked 2 tracked .go file(s)") {
		t.Fatalf("unexpected passing step: %+v", step)
	}
}

func TestValidateFailsClosedWithoutTrackedFilesOrTooling(t *testing.T) {
	cases := map[string]struct {
		deps Deps
		want string
	}{
		"list error": {
			deps: Deps{ListTrackedGoFiles: func(context.Context, string) ([]string, error) { return nil, errors.New("git missing") }},
			want: "list tracked .go files: git missing",
		},
		"no files": {
			deps: Deps{ListTrackedGoFiles: func(context.Context, string) ([]string, error) { return []string{}, nil }},
			want: "no tracked .go files found",
		},
		"gofmt error": {
			deps: Deps{
				ListTrackedGoFiles: func(context.Context, string) ([]string, error) { return []string{"a.go"}, nil },
				ListUnformatted:    func(context.Context, string, []string) ([]string, error) { return nil, errors.New("gofmt missing") },
			},
			want: "gofmt -l: gofmt missing",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			step := ValidateWithDeps("/repo", tc.deps)
			if step.OK || step.Command != Command || !strings.Contains(step.Error, tc.want) {
				t.Fatalf("expected fail-closed step containing %q, got %+v", tc.want, step)
			}
		})
	}
}

func TestValidateAgainstRealGitRepository(t *testing.T) {
	for _, tool := range []string{"git", "gofmt"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "formatted.go"), "package sample\n\nvar Formatted = 1\n")
	writeFile(t, filepath.Join(root, "messy.go"), "package sample\n\nvar   Messy   =   2\n")
	writeFile(t, filepath.Join(root, "untracked.go"), "package sample\n\nvar   Untracked   =   3\n")
	runGit(t, root, "add", "formatted.go", "messy.go")

	step := Validate(root)
	if step.OK || !strings.Contains(step.Error, "messy.go") || strings.Contains(step.Error, "formatted.go") {
		t.Fatalf("expected only tracked messy.go to fail, got %+v", step)
	}
	if strings.Contains(step.Error, "untracked.go") {
		t.Fatalf("untracked files are outside the CI gate and must not be checked: %+v", step)
	}

	if out, err := exec.Command("gofmt", "-w", filepath.Join(root, "messy.go")).CombinedOutput(); err != nil {
		t.Fatalf("gofmt -w failed: %v\n%s", err, out)
	}
	if step := Validate(root); !step.OK {
		t.Fatalf("expected formatted repository to pass, got %+v", step)
	}

	// A tracked file deleted in the working tree (pending git rm) is skipped
	// instead of failing gofmt with an lstat error.
	if err := os.Remove(filepath.Join(root, "formatted.go")); err != nil {
		t.Fatal(err)
	}
	step = Validate(root)
	if !step.OK || !strings.Contains(step.Stdout, "checked 1 tracked .go file(s)") {
		t.Fatalf("expected pending deletion to be skipped, got %+v", step)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
