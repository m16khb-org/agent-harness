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

func TestSealedGitTopologyMutationAllowsOnlyMatchingOriginUpstream(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command string
		want    bool
	}{
		{
			name:    "origin short form",
			command: "git branch --set-upstream-to=origin/190-fitness 190-fitness",
			want:    false,
		},
		{
			name:    "origin full ref form",
			command: "git branch --set-upstream-to refs/remotes/origin/190-fitness 190-fitness",
			want:    false,
		},
		{
			name:    "mismatched upstream branch",
			command: "git branch --set-upstream-to=origin/other 190-fitness",
			want:    true,
		},
		{
			name:    "non origin remote",
			command: "git branch --set-upstream-to=upstream/190-fitness 190-fitness",
			want:    true,
		},
		{
			name:    "implicit local branch",
			command: "git branch --set-upstream-to=origin/190-fitness",
			want:    true,
		},
		{
			name:    "dynamic branch",
			command: "git branch --set-upstream-to=origin/$BRANCH $BRANCH",
			want:    true,
		},
		{
			name:    "rename remains sealed",
			command: "git branch -m renamed",
			want:    true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := SealedGitTopologyMutation(tt.command); got != tt.want {
				t.Fatalf("SealedGitTopologyMutation(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestExactCommitCherryPick(t *testing.T) {
	const commit = "26f6ab35a4d06bc49b452757bea2e7117bca6c8a"
	for _, tt := range []struct {
		name    string
		command string
		want    bool
	}{
		{name: "exact commit", command: "git cherry-pick " + commit, want: true},
		{name: "short commit", command: "git cherry-pick 26f6ab35", want: false},
		{name: "uppercase commit", command: "git cherry-pick 26F6AB35A4D06BC49B452757BEA2E7117BCA6C8A", want: false},
		{name: "commit range", command: "git cherry-pick " + commit + ".." + commit, want: false},
		{name: "multiple commits", command: "git cherry-pick " + commit + " " + commit, want: false},
		{name: "option", command: "git cherry-pick --no-commit " + commit, want: false},
		{name: "absolute git", command: "/usr/bin/git cherry-pick " + commit, want: false},
		{name: "quoted git", command: `"git" cherry-pick ` + commit, want: false},
		{name: "dynamic commit", command: "git cherry-pick $COMMIT", want: false},
		{name: "shell wrapper", command: "sh -c 'git cherry-pick " + commit + "'", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExactCommitCherryPick(tt.command); got != tt.want {
				t.Fatalf("ExactCommitCherryPick(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
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
