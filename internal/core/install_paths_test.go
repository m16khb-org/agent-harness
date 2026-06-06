package core

import (
	"path/filepath"
	"testing"
)

func TestExpandHomeWithHomeExpandsTildeAndPreservesOtherPaths(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "tester")

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "home only", path: "~", want: home},
		{name: "home child", path: "~/skills/self-verify", want: filepath.Join(home, "skills", "self-verify")},
		{name: "other tilde user", path: "~other/project", want: "~other/project"},
		{name: "absolute path", path: filepath.Join(home, "project"), want: filepath.Join(home, "project")},
		{name: "relative path", path: "skills/self-verify", want: "skills/self-verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandHomeWithHome(tt.path, home); got != tt.want {
				t.Fatalf("expandHomeWithHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestHomeRelativePathShortensOnlyHomeDescendants(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "tester")

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "home", path: home, want: "~"},
		{name: "home child", path: filepath.Join(home, "skills", "self-verify"), want: "~/skills/self-verify"},
		{name: "clean home child", path: filepath.Join(home, "skills", "..", "skills", "self-verify"), want: "~/skills/self-verify"},
		{name: "sibling", path: filepath.Join(string(filepath.Separator), "Users", "other"), want: filepath.Join(string(filepath.Separator), "Users", "other")},
		{name: "empty path", path: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := homeRelativePath(tt.path, home); got != tt.want {
				t.Fatalf("homeRelativePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestHomeRelativePathRequiresKnownHome(t *testing.T) {
	path := filepath.Join(string(filepath.Separator), "Users", "tester", "skills")

	if got := homeRelativePath(path, ""); got != path {
		t.Fatalf("homeRelativePath without home = %q, want %q", got, path)
	}
}
