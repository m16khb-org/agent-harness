package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sourceCheckoutRepoForMisdirectTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestSourceCheckoutMisdirectWarningNamesLinkedWorktreeCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := sourceCheckoutRepoForMisdirectTest(t)
	cycle := linkIssueOpsWorktreeForGuardTest(t, repo, "2519-test-quality-comprehensive")
	target := filepath.Join(repo, "src", "a.ts")

	warning := SourceCheckoutMisdirectWarning(HookToolUseLifecycleRequest{
		Repo:  repo,
		Tool:  "apply_patch",
		Paths: []string{target},
	})

	for _, want := range []string{
		cycle.id,
		cycle.path,
		"소스 체크아웃",
		"의도한 대상인지 확인",
		"force-release",
	} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning missing %q:\n%s", want, warning)
		}
	}
}

func TestSourceCheckoutMisdirectWarningSkipsFalseCases(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := sourceCheckoutRepoForMisdirectTest(t)
	cycle := linkIssueOpsWorktreeForGuardTest(t, repo, "2519-test-quality-comprehensive")
	emptyRepo := sourceCheckoutRepoForMisdirectTest(t)

	tests := []struct {
		name string
		req  HookToolUseLifecycleRequest
	}{
		{
			name: "no linked cycle",
			req: HookToolUseLifecycleRequest{
				Repo:  emptyRepo,
				Tool:  "apply_patch",
				Paths: []string{filepath.Join(emptyRepo, "src", "a.ts")},
			},
		},
		{
			name: "target inside linked worktree",
			req: HookToolUseLifecycleRequest{
				Repo:  repo,
				Tool:  "apply_patch",
				Paths: []string{filepath.Join(cycle.path, "src", "a.ts")},
			},
		},
		{
			name: "non mutating tool",
			req: HookToolUseLifecycleRequest{
				Repo:  repo,
				Tool:  "Read",
				Paths: []string{filepath.Join(repo, "src", "a.ts")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if warning := SourceCheckoutMisdirectWarning(tt.req); warning != "" {
				t.Fatalf("expected no warning, got %q", warning)
			}
		})
	}
}
