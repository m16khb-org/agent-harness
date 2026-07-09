package searchrouting

import "testing"

func TestIsShellToolMatchesKnownShellToolNames(t *testing.T) {
	for _, tool := range []string{"Bash", "sh", "zsh", "shell", "exec", "run_command", "shell_command", "unified_exec", "exec_command"} {
		if !isShellTool(tool) {
			t.Fatalf("expected %q to be a shell tool", tool)
		}
	}
	for _, tool := range []string{"", "Read", "mcp__filesystem__read_file", "git"} {
		if isShellTool(tool) {
			t.Fatalf("expected %q to not be a shell tool", tool)
		}
	}
}

func TestSearchTokenNameStripsQuotesAndPath(t *testing.T) {
	cases := map[string]string{
		`"/usr/bin/git"`: "git",
		"'rg'":           "rg",
		"kubectl":        "kubectl",
		"./scripts/x.sh": "x.sh",
	}
	for token, want := range cases {
		if got := searchTokenName(token); got != want {
			t.Fatalf("searchTokenName(%q) = %q, want %q", token, got, want)
		}
	}
}
