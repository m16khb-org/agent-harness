package pathutil

import (
	statecontract "agent-harness/internal/contract/state"
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

func TestHarnessRootPrefersEnvironmentAndReadHarnessFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	if got := HarnessRoot("missing"); got != root {
		t.Fatalf("HarnessRoot env = %q, want %q", got, root)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := ReadHarnessFile(root, "docs", "note.txt")
	if err != nil || text != "hello" {
		t.Fatalf("ReadHarnessFile = %q err=%v", text, err)
	}
}

func TestHarnessRootFromFollowsExecutableSymlink(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join("skills", "atomic-commit-push", "SKILL.md")
	markerPath := filepath.Join(root, marker)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath, []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "agent-harness")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	userBin := filepath.Join(t.TempDir(), ".local", "bin")
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(userBin, "agent-harness")
	short := filepath.Join(userBin, "ah")
	if err := os.Symlink(binary, canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(canonical, short); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	got := harnessRootFrom(marker, "", t.TempDir(), short)
	if got != resolvedRoot {
		t.Fatalf("harnessRootFrom symlink = %q, want %q", got, resolvedRoot)
	}
}

func TestSplitCSVContainsAndStateIssueHelpers(t *testing.T) {
	if Exists(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing path should not exist")
	}
	if got := SplitCSV(" a, ,b,c "); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("SplitCSV = %#v", got)
	}
	if got := SplitCSV(" \t "); len(got) != 0 {
		t.Fatalf("blank SplitCSV = %#v", got)
	}
	if !ContainsString([]string{"a", "b"}, "b") || ContainsString([]string{"a"}, "z") {
		t.Fatal("ContainsString mismatch")
	}
	issues := []statecontract.StateDoctorIssue{{Code: "bad_json"}}
	if !StateDoctorHasIssueCode(issues, "bad_json") || StateDoctorHasIssueCode(issues, "missing") {
		t.Fatal("StateDoctorHasIssueCode mismatch")
	}
}
