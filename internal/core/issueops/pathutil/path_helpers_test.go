package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

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
}
