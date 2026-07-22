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

func TestHasUnquotedBackgroundOperator(t *testing.T) {
	for _, command := range []string{
		`go test ./... 'clean & foreground'`,
		`go test ./... "clean & foreground"`,
		`go test ./... && go vet ./...`,
		`printf x 2>&1`,
		`printf x &>out.log`,
	} {
		if HasUnquotedBackgroundOperator(command) {
			t.Fatalf("foreground or quoted ampersand must not be classified as background: %q", command)
		}
	}
	for _, command := range []string{
		"go test ./... &",
		"go test ./... & git status --short",
	} {
		if !HasUnquotedBackgroundOperator(command) {
			t.Fatalf("unquoted background operator must be rejected: %q", command)
		}
	}
}

func TestHasActiveCommandSubstitution(t *testing.T) {
	for _, command := range []string{
		"orca terminal send --text `touch /tmp/x`",
		`orca terminal send --text "$(touch /tmp/x)"`,
		`rg "` + "`touch /tmp/x`" + `" .`,
		`diff <(git status) <(gh pr create --title x)`,
		`tool --input >(outside-command)`,
	} {
		if !HasActiveCommandSubstitution(command) {
			t.Fatalf("active command substitution must be rejected: %q", command)
		}
	}
	for _, command := range []string{
		`orca terminal send --text 'literal $(not-run) and ` + "`not-run`" + `'`,
		`orca terminal send --text \$(not-run)`,
		"orca terminal send --text \\`not-run\\`",
		`tool --input '<(literal)'`,
		`tool --input "<(literal)"`,
		`tool --input ">(literal)"`,
		`tool --input \<(literal)`,
	} {
		if HasActiveCommandSubstitution(command) {
			t.Fatalf("literal command-substitution data must remain allowed: %q", command)
		}
	}
}

func TestHasActiveOutputRedirect(t *testing.T) {
	for _, command := range []string{
		"rg x > out", "git status >>out", "go test 1>out", "pwd 2>out", "orca task-list 2>>out", "rg x &>out",
	} {
		if !HasActiveOutputRedirect(command) {
			t.Fatalf("active output redirect must be detected: %q", command)
		}
	}
	for _, command := range []string{
		`finish --verification 'literal > evidence'`, `finish --verification "literal >> evidence"`, `rg '\> literal'`, `rg \>literal`,
	} {
		if HasActiveOutputRedirect(command) {
			t.Fatalf("quoted or escaped redirect punctuation must remain data: %q", command)
		}
	}
}

func TestHasActiveInputRedirect(t *testing.T) {
	for _, command := range []string{
		"kubectl exec pod/api -- cat /etc/resolv.conf < /tmp/input",
		"kubectl exec pod/api -- cat /etc/resolv.conf 0</tmp/input",
		"kubectl exec pod/api -- cat /etc/resolv.conf <<<value",
	} {
		if !HasActiveInputRedirect(command) {
			t.Fatalf("active input redirect must be detected: %q", command)
		}
	}
	for _, command := range []string{`printf '%s' '<'`, `printf "%s" "<"`, `printf %s \<`} {
		if HasActiveInputRedirect(command) {
			t.Fatalf("quoted or escaped input punctuation must remain data: %q", command)
		}
	}
}

func TestHasActiveParameterOrTildeExpansion(t *testing.T) {
	for _, command := range []string{
		`rm "$HOME/out"`, `touch ${TMPDIR}/x`, `echo $1`, `echo $?`, `touch ~/x`, `cd ~`,
		`FOO=~/x command`, `FOO=:~/x command`, `FOO=one:~/x command`,
		`env GIT_DIR=~/.git GIT_WORK_TREE=:~/repo git add .`,
	} {
		if !HasActiveParameterOrTildeExpansion(command) {
			t.Fatalf("active parameter/tilde expansion must be detected: %q", command)
		}
	}
	for _, command := range []string{
		`touch '$HOME/literal'`, `touch \$HOME/literal`, `touch \~/literal`, `echo trailing$`,
		`FOO="~/x" command`, `FOO=':~/x' command`, `--label=~/x`,
	} {
		if HasActiveParameterOrTildeExpansion(command) {
			t.Fatalf("literal parameter/tilde text must remain data: %q", command)
		}
	}
}

func TestHasActivePathnameExpansion(t *testing.T) {
	for _, command := range []string{
		`touch {..,inside}/outside.txt`, `chmod {one..three}/target`,
		`touch {..,"inside"}/outside.txt`, `touch {"..",inside}/outside.txt`,
		`touch {1"."."3"}/outside.txt`, `touch ["a"]/outside.txt`,
		`touch o*/pwned`, `rm file?.tmp`, `cp [ab].txt target/`,
	} {
		if !HasActivePathnameExpansion(command) {
			t.Fatalf("active pathname expansion must be detected: %q", command)
		}
	}
	for _, command := range []string{
		`touch '{..,inside}/literal'`, `touch "o*/literal"`, `touch \{one,two\}`, `touch o\*/literal`,
		`touch {one","two}`, `touch {one\,two}`,
		`rg --glob '*.go' pattern`, `printf '%s' '[ab].txt'`,
	} {
		if HasActivePathnameExpansion(command) {
			t.Fatalf("quoted or escaped pathname syntax must remain data: %q", command)
		}
	}
}

func TestSplitCommandTokensCanonicalizesUnquotedBackslashEscapes(t *testing.T) {
	got := SplitCommandTokens(`\g\i\t push origin HEAD --message evidence\ value`)
	want := []string{"git", "push", "origin", "HEAD", "--message", "evidence value"}
	if !stringSlicesEqual(got, want) {
		t.Fatalf("escaped shell argv = %#v, want %#v", got, want)
	}
}

func TestHasActiveShellSpecialQuoting(t *testing.T) {
	for _, command := range []string{`$'git' push origin HEAD`, `$"gh" pr merge 16`, `env $'orca' worktree rm --worktree id:wt-1`} {
		if !HasActiveShellSpecialQuoting(command) {
			t.Fatalf("active shell special quoting must be detected: %q", command)
		}
	}
	for _, command := range []string{`printf '%s' '$'`, `printf '%s' '$"git"'`, `printf %s \$'literal'`} {
		if HasActiveShellSpecialQuoting(command) {
			t.Fatalf("literal special-quote syntax must remain data: %q", command)
		}
	}
}

func TestHasActiveZshEqualsExpansion(t *testing.T) {
	for _, command := range []string{`=git push origin branch`, `tool --input =(print -r -- marker)`} {
		if !HasActiveZshEqualsExpansion(command) {
			t.Fatalf("active zsh equals expansion must be detected: %q", command)
		}
	}
	for _, command := range []string{`printf '%s' '=git'`, `printf '%s' "=(literal)"`, `printf %s \=git`, `NAME=value ./scripts/test.sh`} {
		if HasActiveZshEqualsExpansion(command) {
			t.Fatalf("literal equals text or ordinary assignment must remain allowed: %q", command)
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
