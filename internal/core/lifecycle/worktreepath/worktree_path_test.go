package worktreepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitBranchFromHead(t *testing.T) {
	t.Run("no git dir returns empty", func(t *testing.T) {
		if got := GitBranchFromHead(t.TempDir()); got != "" {
			t.Fatalf("GitBranchFromHead()=%q, want empty", got)
		}
	})

	t.Run("reads branch from HEAD", func(t *testing.T) {
		dir := t.TempDir()
		writeLifecycleGitHead(t, dir, "main")
		if got := GitBranchFromHead(dir); got != "main" {
			t.Fatalf("GitBranchFromHead()=%q, want main", got)
		}
	})

	t.Run("reads branch through linked worktree gitdir file", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, "actual-git-dir")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature/lifecycle\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: actual-git-dir\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := GitBranchFromHead(dir); got != "feature/lifecycle" {
			t.Fatalf("GitBranchFromHead()=%q, want feature/lifecycle", got)
		}
	})

	t.Run("detached or invalid head returns empty", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("abc123\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := GitBranchFromHead(dir); got != "" {
			t.Fatalf("GitBranchFromHead()=%q, want empty", got)
		}
	})

	t.Run("non gitdir file returns empty", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte(strings.Repeat("x", 16)), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := GitBranchFromHead(dir); got != "" {
			t.Fatalf("GitBranchFromHead()=%q, want empty", got)
		}
	})
}

func TestSourceCheckoutFromLinkedWorktreeGitdir(t *testing.T) {
	source := t.TempDir()
	gitDir := filepath.Join(source, ".git", "worktrees", "feature")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourceCheckout(worktree); got != source {
		t.Fatalf("SourceCheckout=%q, want %q", got, source)
	}
	if got := SourceCheckout(source); got != source {
		t.Fatalf("main checkout SourceCheckout=%q, want %q", got, source)
	}
}

func TestCleanAbsAndWithin(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := CleanAbs(" ./nested/../path "), filepath.Join(cwd, "path"); got != want {
		t.Fatalf("CleanAbs()=%q, want %q", got, want)
	}
	if got := CleanAbs(" "); got != "" {
		t.Fatalf("blank CleanAbs()=%q, want empty", got)
	}

	root := t.TempDir()
	child := filepath.Join(root, "child", "file.txt")
	sibling := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-sibling")
	for _, tc := range []struct {
		name string
		path string
		root string
		want bool
	}{
		{name: "root contains itself", path: root, root: root, want: true},
		{name: "root contains descendant", path: child, root: root, want: true},
		{name: "root excludes sibling prefix", path: sibling, root: root, want: false},
		{name: "normalized traversal stays inside", path: filepath.Join(root, "child", "..", "file.txt"), root: root, want: true},
		{name: "blank path is outside", path: "", root: root, want: false},
		{name: "blank root is outside", path: child, root: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Within(tc.path, tc.root); got != tc.want {
				t.Fatalf("Within(%q, %q)=%v, want %v", tc.path, tc.root, got, tc.want)
			}
		})
	}
}

func TestResolveHookTargetPath(t *testing.T) {
	repo := t.TempDir()
	if got, want := ResolveHookTargetPath(repo, " sub/../file.txt "), filepath.Join(repo, "file.txt"); got != want {
		t.Fatalf("relative target=%q, want %q", got, want)
	}
	abs := filepath.Join(t.TempDir(), "file.txt")
	if got := ResolveHookTargetPath(repo, abs); got != abs {
		t.Fatalf("absolute target=%q, want %q", got, abs)
	}
	if got := ResolveHookTargetPath("", "relative.txt"); got != "" {
		t.Fatalf("relative target without repo=%q, want empty", got)
	}
	if got := ResolveHookTargetPath(repo, " "); got != "" {
		t.Fatalf("blank target=%q, want empty", got)
	}
}

func TestIsInsideWorktreesPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/tmp/myproject.worktrees/feature-a", true},
		{"/tmp/.worktrees/something", true},
		{"/tmp/regular/path", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsInsideWorktreesPath(tt.path)
		if got != tt.expected {
			t.Errorf("IsInsideWorktreesPath(%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

func writeLifecycleGitHead(t *testing.T, dir, branch string) {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitWorktreeAddTargets(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"simple path", []string{"../path"}, []string{"../path"}},
		{"with -b flag", []string{"-b", "feature", "../path"}, []string{"../path"}},
		{"with -B flag", []string{"-B", "feature", "../path"}, []string{"../path"}},
		{"with -- separator", []string{"--", "../path"}, []string{"../path"}},
		{"with -b and --", []string{"-b", "feat", "--", "../path"}, []string{"../path"}},
		{"flag only", []string{"--detach"}, nil},
		{"empty", nil, nil},
		{"extra after path ignored", []string{"-b", "feat", "../path", "extra"}, []string{"../path"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gitWorktreeAddTargets(tt.args)
			if !stringSlicesEqual(got, tt.want) {
				t.Errorf("gitWorktreeAddTargets(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestShellCommandGuardPaths(t *testing.T) {
	tests := []struct {
		name    string
		command string
		repo    string
		wantLen int
	}{
		{"cd command", "cd /tmp", t.TempDir(), 1},
		{"git -C", "git -C /tmp status", t.TempDir(), 1},
		{"redirect >", "echo hi > /tmp/out.txt", t.TempDir(), 1},
		{"redirect >>", "echo hi >> /tmp/out.txt", t.TempDir(), 1},
		{"2> redirect", "cmd 2> /tmp/err.txt", t.TempDir(), 1},
		{"inline redirect", "echo hi >/tmp/out.txt 2>/tmp/err.txt", t.TempDir(), 2},
		{"no paths", "echo hello", t.TempDir(), 0},
		{"subshell ignored", "echo $(pwd)", t.TempDir(), 0},
		{"backtick ignored", "echo `date`", t.TempDir(), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShellCommandGuardPaths(tt.repo, tt.command)
			if len(got) != tt.wantLen {
				t.Errorf("ShellCommandGuardPaths(%q) returned %d paths (%v), want %d", tt.command, len(got), got, tt.wantLen)
			}
		})
	}
}

func TestShellCommandGuardPathsExtractsGitRepositoryOverrides(t *testing.T) {
	repo := t.TempDir()
	outside := filepath.Join(t.TempDir(), "source")
	wantGitDir := filepath.Join(outside, ".git")
	tests := []struct {
		name    string
		command string
	}{
		{name: "global equals flags", command: "git --git-dir=" + wantGitDir + " --work-tree=" + outside + " add ."},
		{name: "global split flags", command: "git --git-dir " + wantGitDir + " --work-tree " + outside + " add ."},
		{name: "direct environment", command: "GIT_DIR=" + wantGitDir + " GIT_WORK_TREE=" + outside + " git add ."},
		{name: "env wrapper", command: "env GIT_DIR=" + wantGitDir + " GIT_WORK_TREE=" + outside + " git add ."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShellCommandGuardPaths(repo, tt.command)
			if !containsGuardPath(got, wantGitDir) || !containsGuardPath(got, outside) {
				t.Fatalf("Git repository overrides were not extracted: command=%q paths=%v", tt.command, got)
			}
		})
	}
}

func containsGuardPath(paths []string, want string) bool {
	for _, path := range paths {
		if CleanAbs(path) == CleanAbs(want) {
			return true
		}
	}
	return false
}

func TestIssueOpsPreparationCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{"worktree add into worktrees", "git worktree add -b feature /tmp/proj.worktrees/feat", true},
		{"worktree add with .worktrees in path", "git worktree add ../some.worktrees/feat", true},
		{"regular worktree add", "git worktree add -b feature /tmp/normal/path", false},
		{"not worktree", "git status", false},
		{"not git", "echo hello", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IssueOpsPreparationCommand(tt.command)
			if got != tt.expected {
				t.Errorf("IssueOpsPreparationCommand(%q) = %v, want %v", tt.command, got, tt.expected)
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
