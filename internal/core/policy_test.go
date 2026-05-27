package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandPolicyAllowsReadOnlyInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	result := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"git", "status", "--short"},
		Timeout:       "30s",
	})
	if !result.OK || !result.Allowed {
		t.Fatalf("policy denied read-only command: %+v", result)
	}
}

func TestCommandPolicyDeniesOutsideWorkspaceAndShell(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideResult := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           outside,
		Argv:          []string{"git", "status", "--short"},
		Timeout:       "30s",
	})
	if outsideResult.Allowed || !containsString(outsideResult.DenyReasons, "cwd_outside_workspace") {
		t.Fatalf("outside cwd not denied: %+v", outsideResult)
	}

	shellResult := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"sh", "-c", "echo ok"},
		Timeout:       "30s",
	})
	if shellResult.Allowed || !containsString(shellResult.DenyReasons, "shell_interpreter_not_allowed") {
		t.Fatalf("shell command not denied: %+v", shellResult)
	}
}

func TestCommandPolicyDeniesPathArgsOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "note.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", outside)
	link := filepath.Join(inside, "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		argv []string
	}{
		{name: "relative parent escape", argv: []string{"cat", filepath.Join("..", "..", filepath.Base(outside), "note.txt")}},
		{name: "absolute outside path", argv: []string{"cat", outsideFile}},
		{name: "flag value outside path", argv: []string{"sed", "--file=" + outsideFile}},
		{name: "symlink escape path", argv: []string{"cat", filepath.Join(link, "note.txt")}},
		{name: "home shorthand escape", argv: []string{"cat", "~/note.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := EvaluateCommandPolicy(CommandPolicyRequest{
				WorkspaceRoot: root,
				CWD:           inside,
				Argv:          tc.argv,
				Timeout:       "30s",
			})
			if result.Allowed || !containsString(result.DenyReasons, "path_outside_workspace") {
				t.Fatalf("outside path arg not denied: %+v", result)
			}
		})
	}

	insideResult := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           inside,
		Argv:          []string{"cat", filepath.Join("..", "inside", "local.txt")},
		Timeout:       "30s",
	})
	if !insideResult.Allowed {
		t.Fatalf("inside path arg should be allowed: %+v", insideResult)
	}
}

func TestPolicyPathCandidatesIgnoreRemoteReferences(t *testing.T) {
	for _, arg := range []string{
		"https://github.com/example/repo",
		"ssh://git@github.com/example/repo",
		"git@github.com:example/repo",
	} {
		if got := policyPathCandidates(arg); len(got) != 0 {
			t.Fatalf("remote reference %q should not be treated as a local path: %+v", arg, got)
		}
	}
	if got := policyPathCandidates("--file=/tmp/outside"); len(got) != 1 || got[0] != "/tmp/outside" {
		t.Fatalf("flag path candidate not detected: %+v", got)
	}
}

func TestCommandFakeRunDoesNotExecute(t *testing.T) {
	root := t.TempDir()
	result := FakeRunCommand(CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"touch", "marker"},
		Timeout:       "30s",
		WriteAllowed:  true,
	})
	if !result.OK || result.Executed || !result.Policy.Allowed {
		t.Fatalf("unexpected fake-run result: %+v", result)
	}
	if existsForTest(filepath.Join(root, "marker")) {
		t.Fatalf("fake-run created marker; command executed")
	}
}

func TestCommandPolicyRedactsAndDeniesSecretLikeArgs(t *testing.T) {
	root := t.TempDir()
	result := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           root,
		Argv:          []string{"cat", ".env"},
		Timeout:       "30s",
	})
	if result.Allowed || !containsString(result.DenyReasons, "secret_like_argument") {
		t.Fatalf("secret-like arg not denied: %+v", result)
	}
	if len(result.Argv) < 2 || result.Argv[1] != "<redacted>" {
		t.Fatalf("secret-like arg not redacted: %+v", result.Argv)
	}
}

func TestCommandPolicyCatalogDrivesAllowAndDenyDecisions(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name   string
		argv   []string
		flags  func(*CommandPolicyRequest)
		reason string
	}{
		{name: "read-only command", argv: []string{"rg", "needle", "."}},
		{name: "read-only git subcommand", argv: []string{"git", "diff", "--stat"}},
		{name: "read-only go subcommand", argv: []string{"go", "env"}},
		{name: "network command", argv: []string{"curl", "https://example.invalid"}, reason: "network_not_allowed"},
		{name: "network git subcommand", argv: []string{"git", "fetch"}, reason: "network_not_allowed"},
		{name: "write command", argv: []string{"touch", "marker"}, reason: "write_not_allowed"},
		{name: "write git subcommand", argv: []string{"git", "commit", "-m", "test"}, reason: "write_not_allowed"},
		{name: "write go subcommand", argv: []string{"go", "test", "./..."}, reason: "write_not_allowed"},
		{
			name: "write command allowed by flag",
			argv: []string{"touch", "marker"},
			flags: func(req *CommandPolicyRequest) {
				req.WriteAllowed = true
			},
		},
		{
			name: "network command allowed by flag",
			argv: []string{"curl", "https://example.invalid"},
			flags: func(req *CommandPolicyRequest) {
				req.NetworkAllowed = true
				req.WriteAllowed = true // curl is outside the read-only allowlist, so explicit write/process approval is also required.
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := CommandPolicyRequest{
				WorkspaceRoot: root,
				CWD:           root,
				Argv:          tc.argv,
				Timeout:       "30s",
			}
			if tc.flags != nil {
				tc.flags(&req)
			}
			result := EvaluateCommandPolicy(req)
			if tc.reason == "" {
				if !result.Allowed {
					t.Fatalf("policy denied %v: %+v", tc.argv, result)
				}
				return
			}
			if result.Allowed || !containsString(result.DenyReasons, tc.reason) {
				t.Fatalf("policy did not deny %v with %s: %+v", tc.argv, tc.reason, result)
			}
		})
	}
}

func TestCommandPolicySummaryIncludesCatalog(t *testing.T) {
	summary := CommandPolicySummary()
	catalog, ok := summary["catalog"].(map[string]any)
	if !ok {
		t.Fatalf("summary catalog missing: %+v", summary)
	}
	required := []string{"read_only_commands", "read_only_subcommands", "write_commands", "write_subcommands", "network_commands", "network_subcommands", "shell_interpreters"}
	for _, key := range required {
		if _, ok := catalog[key]; !ok {
			t.Fatalf("summary catalog missing %s: %+v", key, catalog)
		}
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func existsForTest(path string) bool {
	return exists(path)
}
