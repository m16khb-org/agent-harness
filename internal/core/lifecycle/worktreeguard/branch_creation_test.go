package worktreeguard

import (
	"testing"
)

func TestLocalIssueOpsBranchCreation(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    BranchCreation
	}{
		{
			name:    "git checkout -b",
			command: `git checkout -b feature/branch origin/main`,
			want:    BranchCreation{Branch: "feature/branch", SourceRef: "origin/main"},
		},
		{
			name:    "git checkout -B force",
			command: `git checkout -B feature/branch origin/main`,
			want:    BranchCreation{Branch: "feature/branch", SourceRef: "origin/main"},
		},
		{
			name:    "git switch -c",
			command: `git switch -c feature/branch origin/main`,
			want:    BranchCreation{Branch: "feature/branch", SourceRef: "origin/main"},
		},
		{
			name:    "git switch --create",
			command: `git switch --create feature/branch origin/main`,
			want:    BranchCreation{Branch: "feature/branch", SourceRef: "origin/main"},
		},
		{
			name:    "git switch --create=value",
			command: `git switch --create=feature/branch origin/main`,
			want:    BranchCreation{Branch: "feature/branch", SourceRef: "origin/main"},
		},
		{
			name:    "git worktree add -b",
			command: `git worktree add -b feature/branch ../path origin/main`,
			want:    BranchCreation{Branch: "feature/branch", SourceRef: "origin/main"},
		},
		{
			name:    "git worktree add -b with --",
			command: `git worktree add -b feature/branch -- ../path origin/main`,
			want:    BranchCreation{Branch: "feature/branch", SourceRef: "origin/main"},
		},
		{
			name:    "git worktree add -b no source ref",
			command: `git worktree add -b feature/branch ../path`,
			want:    BranchCreation{Branch: "feature/branch"},
		},
		{
			name:    "empty",
			command: ``,
			want:    BranchCreation{},
		},
		{
			name:    "not git",
			command: `echo hello`,
			want:    BranchCreation{},
		},
		{
			name:    "git without branch flag",
			command: `git checkout main`,
			want:    BranchCreation{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LocalIssueOpsBranchCreation(tt.command)
			if got.Branch != tt.want.Branch || got.SourceRef != tt.want.SourceRef {
				t.Errorf("LocalIssueOpsBranchCreation(%q) = %+v, want %+v", tt.command, got, tt.want)
			}
		})
	}
}

func TestShellTokenLooksDynamic(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"$HOME", true},
		{"`date`", true},
		{"plain", false},
		{`"$PATH"`, true},
		{"", false},
	}
	for _, tt := range tests {
		got := ShellTokenLooksDynamic(tt.token)
		if got != tt.expected {
			t.Errorf("ShellTokenLooksDynamic(%q) = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

func TestLocalIssueOpsBranchSelection(t *testing.T) {
	for _, tt := range []struct {
		command string
		want    BranchSelection
	}{
		{`git checkout 123-demo`, BranchSelection{Branch: "123-demo"}},
		{`git switch 123-demo`, BranchSelection{Branch: "123-demo"}},
		{`git checkout -- internal/x.go`, BranchSelection{}},
		{`git switch -c 123-demo origin/main`, BranchSelection{}},
		{`git checkout "$BRANCH"`, BranchSelection{Branch: "$BRANCH", Dynamic: true}},
	} {
		if got := LocalIssueOpsBranchSelection(tt.command); got != tt.want {
			t.Errorf("LocalIssueOpsBranchSelection(%q) = %+v, want %+v", tt.command, got, tt.want)
		}
	}
}

func TestIssueOpsBranchCreationSourceReason(t *testing.T) {
	reason := IssueOpsBranchCreationSourceReason("feature/foo")
	if reason == "" {
		t.Error("expected non-empty reason")
	}
	if reason[:35] != "IssueOps branch creation must inclu" {
		t.Errorf("unexpected reason: %q", reason)
	}
}
