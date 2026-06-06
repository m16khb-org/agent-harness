package pathutil

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveTargetPrefersExplicitArgument(t *testing.T) {
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())
	t.Setenv("PWD", t.TempDir())
	explicit := t.TempDir()

	got := ResolveTarget(explicit)
	want, err := filepath.Abs(explicit)
	if err != nil {
		t.Fatalf("abs explicit target: %v", err)
	}

	if got != want {
		t.Fatalf("resolveTarget() = %q, want %q", got, want)
	}
}

func TestResolveTargetUsesEnvironmentFallbacksWhenArgumentIsEmpty(t *testing.T) {
	claudeDir := t.TempDir()
	pwdDir := t.TempDir()

	t.Run("claude project dir wins", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", claudeDir)
		t.Setenv("PWD", pwdDir)

		got := ResolveTarget("")
		want, err := filepath.Abs(claudeDir)
		if err != nil {
			t.Fatalf("abs claude dir: %v", err)
		}

		if got != want {
			t.Fatalf("resolveTarget() = %q, want %q", got, want)
		}
	})

	t.Run("pwd is used without claude project dir", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", "")
		t.Setenv("PWD", pwdDir)

		got := ResolveTarget("")
		want, err := filepath.Abs(pwdDir)
		if err != nil {
			t.Fatalf("abs pwd dir: %v", err)
		}

		if got != want {
			t.Fatalf("resolveTarget() = %q, want %q", got, want)
		}
	})
}

func TestResolveTargetUsesCurrentDirectoryWhenEnvironmentFallbacksAreEmpty(t *testing.T) {
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(oldCWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})

	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir temp cwd: %v", err)
	}
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	t.Setenv("PWD", "")

	got := ResolveTarget("")
	want, err := os.Getwd()
	if err != nil {
		t.Fatalf("get temp cwd: %v", err)
	}

	if got != want {
		t.Fatalf("resolveTarget() = %q, want %q", got, want)
	}
}

func TestSplitLinesPreservesInteriorBlankLinesAndDropsTrailingNewlines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: []string{}},
		{name: "whitespace only", in: " \n\t", want: []string{}},
		{name: "trailing newline", in: "one\ntwo\n", want: []string{"one", "two"}},
		{name: "repeated trailing newline", in: "one\n\n", want: []string{"one"}},
		{name: "interior blank line", in: "one\n\n two", want: []string{"one", "", " two"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitLines(tc.in)

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitLines(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFindUpReturnsNearestAncestorWithMarker(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested dirs: %v", err)
	}
	marker := filepath.Join("skills", "atomic-commit-push", "SKILL.md")
	markerPath := filepath.Join(root, marker)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		t.Fatalf("create marker parent: %v", err)
	}
	if err := os.WriteFile(markerPath, []byte("skill"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	got, ok := FindUp(nested, marker)

	if !ok || got != root {
		t.Fatalf("findUp() = (%q, %v), want (%q, true)", got, ok, root)
	}
}

func TestFindUpReturnsFalseWhenMarkerIsMissing(t *testing.T) {
	got, ok := FindUp(t.TempDir(), filepath.Join("missing", "marker"))

	if ok || got != "" {
		t.Fatalf("findUp() = (%q, %v), want (\"\", false)", got, ok)
	}
}
