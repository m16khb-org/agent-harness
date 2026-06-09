package worktreepath

import (
	"testing"
)

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
