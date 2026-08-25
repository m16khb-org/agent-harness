package repoidentity

import (
	"path/filepath"
	"testing"
)

func TestSourceRootMapsGitCommonDirToPrimaryCheckout(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		commonDir string
		want      string
	}{
		{
			name: "empty path stays empty",
			path: "",
			want: "",
		},
		{
			name: "whitespace path stays empty",
			path: "   ",
			want: "",
		},
		{
			name: "empty common dir returns cleaned caller path",
			path: "/tmp/workspace/repo",
			want: "/tmp/workspace/repo",
		},
		{
			name:      "absolute git common dir maps to checkout dir",
			path:      "/tmp/worktree",
			commonDir: "/tmp/primary/repo/.git",
			want:      "/tmp/primary/repo",
		},
		{
			name:      "common dir without .git base keeps caller path",
			path:      "/var/folders/worktree",
			commonDir: "/private/var/folders/primary",
			want:      "/var/folders/worktree",
		},
		{
			name:      "dot git suffix directory is not treated as git dir",
			path:      "/tmp/workspace/repo",
			commonDir: "/tmp/workspace/repo/my.git",
			want:      "/tmp/workspace/repo",
		},
		{
			name:      "relative common dir joins caller path",
			path:      "/var/folders/worktree",
			commonDir: ".git",
			want:      "/var/folders/worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SourceRoot(tt.path, tt.commonDir)
			if got != tt.want {
				t.Fatalf("SourceRoot(%q, %q) = %q, want %q", tt.path, tt.commonDir, got, tt.want)
			}
		})
	}
}

func TestSourceRootCleansRelativeCallerPath(t *testing.T) {
	abs, err := filepath.Abs("repo/../repo")
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	if got := SourceRoot("repo/../repo", ""); got != abs {
		t.Fatalf("SourceRoot relative path = %q, want cleaned absolute %q", got, abs)
	}
}
