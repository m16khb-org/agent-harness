package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanAbsPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got := CleanAbsPath(" ./nested/../path ")
	want := filepath.Join(cwd, "path")
	if got != want {
		t.Fatalf("CleanAbsPath returned %q, want %q", got, want)
	}
	if got := CleanAbsPath(" \t "); got != "" {
		t.Fatalf("blank path returned %q, want empty", got)
	}
}

func TestPathWithin(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child", "file.txt")
	sibling := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-sibling")

	tests := []struct {
		name string
		path string
		root string
		want bool
	}{
		{name: "root contains itself", path: root, root: root, want: true},
		{name: "root contains descendants", path: child, root: root, want: true},
		{name: "sibling with same prefix is outside", path: sibling, root: root, want: false},
		{name: "parent traversal normalizes inside", path: filepath.Join(root, "child", "..", "file.txt"), root: root, want: true},
		{name: "blank path is outside", path: "", root: root, want: false},
		{name: "blank root is outside", path: child, root: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PathWithin(tt.path, tt.root); got != tt.want {
				t.Fatalf("PathWithin(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.want)
			}
		})
	}
}

func TestIsInsideWorktreesPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/tmp/project.worktrees/feat", true},
		{"/tmp/.worktrees/a", true},
		{"/tmp/normal", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsInsideWorktreesPath(tt.path)
		if got != tt.expected {
			t.Errorf("IsInsideWorktreesPath(%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

func TestGitBranchFromHead(t *testing.T) {
	t.Run("no git dir returns empty", func(t *testing.T) {
		dir := t.TempDir()
		got := GitBranchFromHead(dir)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("reads branch from HEAD", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.MkdirAll(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
		got := GitBranchFromHead(dir)
		if got != "main" {
			t.Errorf("expected 'main', got %q", got)
		}
	})

	t.Run("detached HEAD returns empty", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.MkdirAll(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("abc123def456\n"), 0o644)
		got := GitBranchFromHead(dir)
		if got != "" {
			t.Errorf("expected empty for detached HEAD, got %q", got)
		}
	})

	t.Run("reads branch through linked worktree gitdir file", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, "actual-git-dir")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature/pathutil\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: actual-git-dir\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		got := GitBranchFromHead(dir)
		if got != "feature/pathutil" {
			t.Errorf("expected linked worktree branch, got %q", got)
		}
	})

	t.Run("non gitdir file falls back to missing HEAD", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte(strings.Repeat("x", 16)), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := GitBranchFromHead(dir); got != "" {
			t.Errorf("expected empty for non-gitdir .git file, got %q", got)
		}
	})
}
