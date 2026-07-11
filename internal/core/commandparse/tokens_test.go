package commandparse

import "testing"

func TestSplitCommandTokens(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"empty", "", nil},
		{"single word", "hello", []string{"hello"}},
		{"two words", "hello world", []string{"hello", "world"}},
		{"double quoted", `echo "hello world"`, []string{"echo", "hello world"}},
		{"single quoted", "echo 'hello world'", []string{"echo", "hello world"}},
		{"mixed quotes", `'it' "works"`, []string{"it", "works"}},
		{"escaped newline", `"line1\nline2"`, []string{"line1\nline2"}},
		{"escaped tab", `"col1\tcol2"`, []string{"col1\tcol2"}},
		{"escaped return", `"a\rb"`, []string{"a\rb"}},
		{"literal backslash n", `"\\n"`, []string{"\\n"}},
		{"tabs between", "one\ttwo", []string{"one", "two"}},
		{"newlines between", "one\ntwo", []string{"one", "two"}},
		{"multiple spaces", "a   b", []string{"a", "b"}},
		{"gh issue with flags", `gh issue create --title "버그 수정" --body "내용"`, []string{"gh", "issue", "create", "--title", "버그 수정", "--body", "내용"}},
		{"flag with equals", `--labels="bug,feature"`, []string{"--labels=bug,feature"}},
		{"unclosed quote", `"unclosed`, []string{"unclosed"}},
		{"glab command", `glab mr create --title "fix"`, []string{"glab", "mr", "create", "--title", "fix"}},
		{"git worktree", `git worktree add -b feature ../path`, []string{"git", "worktree", "add", "-b", "feature", "../path"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitCommandTokens(tt.command)
			if !stringSlicesEqual(got, tt.want) {
				t.Errorf("SplitCommandTokens(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestHasUnquotedControlOperator(t *testing.T) {
	for _, command := range []string{
		`agent-harness finish --verification 'exact; clean & verified | complete'`,
		`agent-harness finish --verification "exact; clean & verified | complete"`,
	} {
		if HasUnquotedControlOperator(command) {
			t.Fatalf("quoted evidence punctuation must be data: %q", command)
		}
	}
	for _, command := range []string{
		"agent-harness finish; touch x",
		"agent-harness finish & touch x",
		"agent-harness finish | touch x",
		"agent-harness finish\ntouch x",
		"agent-harness finish\rtouch x",
	} {
		if !HasUnquotedControlOperator(command) {
			t.Fatalf("unquoted control operator must be rejected: %q", command)
		}
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
