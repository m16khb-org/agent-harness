package commandparse

import "testing"

// TestParseExactIssueOpsCommandCorpus is the accept/reject characterization
// corpus for the exact-issueops parser extracted from the lifecycle authority
// layer (Task C). It pins parsing behavior at the new home so the move stays
// byte-identical.
func TestParseExactIssueOpsCommandCorpus(t *testing.T) {
	cases := []struct {
		command  string
		wantOK   bool
		wantPath string
	}{
		{"agent-harness issueops status --id io-1 --json", true, "status"},
		{"agent-harness issueops resume --repo /r --id io-1", true, "resume"},
		{"./bin/agent-harness issueops handoff claim --id io-1", true, "handoff claim"},
		{"agent-harness issueops handoff start --id io-1", true, "handoff start"},
		{"agent-harness issueops worktree prepare --id io-1", true, "worktree prepare"},
		{"agent-harness issueops compatibility review --id io-1", true, "compatibility review"},
		{"agent-harness issueops phase --id io-1 --to done", true, "phase"},
		// Two-word subcommand with a flag where the second word is missing -> reject.
		{"agent-harness issueops handoff --id io-1", false, ""},
		{"agent-harness issueops", false, ""},
		{"git status", false, ""},
		{"agent-harness build", false, ""},
		// Active shell control / expansion must fail closed.
		{"agent-harness issueops status --id io-1; rm -rf /", false, ""},
		{"agent-harness issueops status --id $(whoami)", false, ""},
		{"agent-harness issueops status --id io-1 > out.txt", false, ""},
		{"", false, ""},
	}
	for _, tc := range cases {
		got, ok := ParseExactIssueOpsCommand(tc.command)
		if ok != tc.wantOK {
			t.Fatalf("ParseExactIssueOpsCommand(%q) ok=%v want=%v", tc.command, ok, tc.wantOK)
		}
		if ok && got.Path != tc.wantPath {
			t.Fatalf("ParseExactIssueOpsCommand(%q) path=%q want=%q", tc.command, got.Path, tc.wantPath)
		}
	}
}

func TestParseExactIssueOpsCommandAcceptsRepoLocalBinSpelling(t *testing.T) {
	parsed, ok := ParseExactIssueOpsCommand("bin/agent-harness issueops status --id io-1 --json")
	if !ok || parsed.Path != "status" {
		t.Fatalf("repo-local bin spelling must parse exactly: parsed=%#v ok=%v", parsed, ok)
	}
	if _, ok := ParseExactIssueOpsCommand("bin/agent-harness issueops status --id io-1; rm -rf /"); ok {
		t.Fatal("repo-local bin spelling must still reject active shell control")
	}
}

func TestExactFlagsCorpus(t *testing.T) {
	spec := func(path string) (map[string]bool, map[string]bool, map[string]bool) {
		v, b, r, ok := IssueOpsCommandSpec(path)
		if !ok {
			t.Fatalf("missing spec for %q", path)
		}
		return v, b, r
	}
	// A flag token must not become another flag's value.
	cmd, _ := ParseExactIssueOpsCommand("agent-harness issueops handoff claim --agent-id --cwd /w")
	v, b, r := spec(cmd.Path)
	if _, ok := ExactFlags(cmd, v, b, r); ok {
		t.Fatal("flag token must not be consumed as a value")
	}
	// Unknown flag rejected.
	cmd2, _ := ParseExactIssueOpsCommand("agent-harness issueops status --id io-1 --unknown x")
	v2, b2, r2 := spec(cmd2.Path)
	if _, ok := ExactFlags(cmd2, v2, b2, r2); ok {
		t.Fatal("unknown flag must be rejected")
	}
	// Repeatable flag accepted multiple times; non-repeatable rejected twice.
	cmd3, _ := ParseExactIssueOpsCommand("agent-harness issueops handoff start --id io-1 --criteria-id A --criteria-id B")
	v3, b3, r3 := spec(cmd3.Path)
	if flags, ok := ExactFlags(cmd3, v3, b3, r3); !ok || len(flags["--criteria-id"]) != 2 {
		t.Fatalf("repeatable flag not accepted twice: ok=%v flags=%#v", ok, flags)
	}
	cmd4, _ := ParseExactIssueOpsCommand("agent-harness issueops status --id io-1 --id io-2")
	v4, b4, r4 := spec(cmd4.Path)
	if _, ok := ExactFlags(cmd4, v4, b4, r4); ok {
		t.Fatal("duplicate non-repeatable flag must be rejected")
	}
	// The long-form legacy spelling remains valid because older coordinator
	// packets emitted it. It must not fall back to multi-cycle ambiguity.
	cmd5, _ := ParseExactIssueOpsCommand("agent-harness issueops handoff start --id io-1 --verification-command go-test --verification-command go-vet")
	v5, b5, r5 := spec(cmd5.Path)
	if flags, ok := ExactFlags(cmd5, v5, b5, r5); !ok || len(flags["--verification-command"]) != 2 {
		t.Fatalf("legacy verification-command alias not accepted: ok=%v flags=%#v", ok, flags)
	}
}

func TestReadyWorkspaceCheckpointSpecsAcceptNativeActorFlags(t *testing.T) {
	commands := []string{
		"agent-harness issueops link-plan --id io-1 --plan-path /w/plans/io-1.md",
		"agent-harness issueops compatibility review --id io-1 --approved",
		"agent-harness issueops execution decide --id io-1 --subagent-use none",
		"agent-harness issueops devils-advocate review --id io-1 --verdict pass",
		"agent-harness issueops worktree prepare-tools --id io-1",
	}
	for _, base := range commands {
		command, ok := ParseExactIssueOpsCommand(base + " --host codex --session-id session-1 --agent-id agent-1 --cwd /repo --json")
		if !ok {
			t.Fatalf("checkpoint command did not parse: %q", base)
		}
		values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
		if !ok {
			t.Fatalf("checkpoint command has no exact spec: %q", command.Path)
		}
		if _, ok := ExactFlags(command, values, booleans, repeatable); !ok {
			t.Fatalf("checkpoint actor flags rejected for %q", command.Path)
		}
	}
}

func TestExactReadOnlyShellCommandCorpus(t *testing.T) {
	allow := []string{
		"pwd",
		"rg -n handoff internal",
		"rg --files --hidden",
		"git status --short",
		"git -C /repo diff --stat",
		"git log -1",
		"orca terminal list --json",
		"orca terminal wait --terminal t --for exit --json",
		"orca orchestration task-list --json",
	}
	deny := []string{
		"pwd extra",
		"rg --pre danger pattern",
		"git status -o out.txt",
		"git push",
		"git commit -m x",
		"orca terminal send --terminal t --text x --json",
		"orca terminal wait --terminal t --for spin --json",
		"rm -rf /",
		"rg handoff > out.txt",
		"cat file",
	}
	for _, c := range allow {
		if !ExactReadOnlyShellCommand(c) {
			t.Fatalf("expected read-only allow: %q", c)
		}
	}
	for _, c := range deny {
		if ExactReadOnlyShellCommand(c) {
			t.Fatalf("expected read-only deny: %q", c)
		}
	}
}

func TestExactReadOnlyShellCommandAllowsOnlyExactCodeGraphExplore(t *testing.T) {
	allow := []string{
		"codegraph explore 'handoff ownership path'",
		`codegraph explore "lifecycleRecordID"`,
	}
	deny := []string{
		"codegraph explore",
		"codegraph explore -q",
		"codegraph explore --path /tmp query",
		"codegraph explore one two",
		"codegraph sync query",
		"./codegraph explore query",
		"codegraph explore query > out.txt",
		"codegraph explore </tmp/input",
		"codegraph explore 0</tmp/input",
		"codegraph explore <<<value",
		"codegraph explore $(whoami)",
	}
	for _, command := range allow {
		if !ExactReadOnlyShellCommand(command) {
			t.Fatalf("expected exact CodeGraph observation allow: %q", command)
		}
	}
	for _, command := range deny {
		if ExactReadOnlyShellCommand(command) {
			t.Fatalf("expected inexact CodeGraph command deny: %q", command)
		}
	}
}

func TestSafeRipgrepArgsCorpus(t *testing.T) {
	safe := [][]string{
		{"-n", "pattern"},
		{"--glob", "*.go", "pattern"},
		{"-C", "3", "pattern"},
		{"--type=go", "pattern"},
		{"pattern"},
	}
	unsafe := [][]string{
		{"--pre", "danger"},
		{"-C"},        // value flag missing its value
		{"--glob"},    // value flag missing its value
		{"--unknown"}, // unknown flag
		{"-g", "-x"},  // value flag followed by another flag
	}
	for _, a := range safe {
		if !SafeRipgrepArgs(a) {
			t.Fatalf("expected safe rg args: %v", a)
		}
	}
	for _, a := range unsafe {
		if SafeRipgrepArgs(a) {
			t.Fatalf("expected unsafe rg args: %v", a)
		}
	}
}

func TestContainsASCIITerminalControlCorpus(t *testing.T) {
	if ContainsASCIITerminalControl("plain guidance text") {
		t.Fatal("plain text must not flag")
	}
	for _, s := range []string{"a\x1bb", "a\tb", "a\x7f", "a\nb", "a\rb"} {
		if !ContainsASCIITerminalControl(s) {
			t.Fatalf("control char not detected in %q", s)
		}
	}
}

func TestCommandAfterDirectoryOptionCorpus(t *testing.T) {
	if got := CommandAfterDirectoryOption([]string{"git", "-C", "/r", "status"}, 1); got != 3 {
		t.Fatalf("expected index 3 after -C dir, got %d", got)
	}
	if got := CommandAfterDirectoryOption([]string{"git", "status"}, 1); got != 1 {
		t.Fatalf("expected index 1 with no -C, got %d", got)
	}
	if got := CommandAfterDirectoryOption([]string{"git", "-C"}, 1); got != -1 {
		t.Fatalf("expected -1 for malformed -C, got %d", got)
	}
	if got := CommandAfterDirectoryOption([]string{"git", "-C=", "status"}, 1); got != -1 {
		t.Fatalf("expected -1 for empty -C= value, got %d", got)
	}
}
