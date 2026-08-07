package readinesspaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreePathValid(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"valid dir", dir, true},
		{"missing", "/nonexistent", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreePathValid(tt.path)
			if got != tt.expected {
				t.Errorf("WorktreePathValid(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPlanPathExists(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	os.WriteFile(planPath, []byte("content"), 0o644)

	tests := []struct {
		name     string
		repo     string
		path     string
		expected bool
	}{
		{"absolute file exists", "", planPath, true},
		{"relative inside repo", dir, "plan.md", true},
		{"missing file", dir, "nope.md", false},
		{"empty path", dir, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanPathExists(tt.repo, tt.path)
			if got != tt.expected {
				t.Errorf("PlanPathExists(%q, %q) = %v, want %v", tt.repo, tt.path, got, tt.expected)
			}
		})
	}
}

func TestPlanPathInsideWorktree(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)

	tests := []struct {
		name     string
		worktree string
		planPath string
		expected bool
	}{
		{"relative plan inside", dir, "plan.md", true},
		{"absolute subdir inside", dir, sub, true},
		{"empty path rejected", dir, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanPathInsideWorktree(tt.worktree, tt.planPath)
			if got != tt.expected {
				t.Errorf("PlanPathInsideWorktree(%q, %q) = %v, want %v", tt.worktree, tt.planPath, got, tt.expected)
			}
		})
	}
}
