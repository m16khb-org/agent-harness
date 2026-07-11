package policy

import (
	"os"
	"strings"
	"testing"
)

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

func TestRedactFreeformRecognizesAuthorizationBearerHeaderWithOpaqueValue(t *testing.T) {
	const opaque = "opaque-bearer-value-7F3A"
	if got := RedactFreeform("Authorization: Bearer " + opaque); got != "<redacted>" {
		t.Fatalf("Authorization Bearer header was not redacted: %q", got)
	}
	if got := RedactFreeform("api_key=" + opaque); got != "<redacted>" {
		t.Fatalf("api_key assignment was not redacted independently: %q", got)
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

func TestPolicyOverridesLoadPerEvaluation(t *testing.T) {
	repoRoot := t.TempDir()
	agentHarnessDir := repoRoot + "/.agent-harness"
	if err := os.MkdirAll(agentHarnessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	policyJSON := `{
		"additional_shell_interpreters": ["tsh"],
		"additional_network_commands": ["nc"],
		"additional_network_subcommands": {"git": ["archive"]},
		"additional_write_commands": ["tr"],
		"additional_write_subcommands": {"git": ["tag"]},
		"additional_read_only_commands": ["echo"],
		"additional_read_only_subcommands": {"git": ["stash"]}
	}`
	if err := os.WriteFile(agentHarnessDir+"/policy.json", []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	result := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: repoRoot,
		CWD:           repoRoot,
		Argv:          []string{"echo", "ok"},
		Timeout:       "30s",
	})
	if !result.Allowed {
		t.Fatalf("expected additional read-only command to be allowed: %+v", result)
	}

	network := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot:  repoRoot,
		CWD:            repoRoot,
		Argv:           []string{"nc", "example.invalid", "80"},
		Timeout:        "30s",
		NetworkAllowed: true,
		WriteAllowed:   true,
	})
	if !network.Allowed {
		t.Fatalf("expected additional network command to be recognized and allowed with flags: %+v", network)
	}

	shell := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: repoRoot,
		CWD:           repoRoot,
		Argv:          []string{"bash", "-c", "true"},
		Timeout:       "30s",
	})
	if !containsString(shell.DenyReasons, "shell_interpreter_not_allowed") {
		t.Fatalf("built-in shell interpreter classification was lost: %+v", shell)
	}
}

func TestLoadPolicyOverridesNoFileIsBackwardCompatible(t *testing.T) {
	repoRoot := t.TempDir()
	result := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: repoRoot,
		CWD:           repoRoot,
		Argv:          []string{"git", "status", "--short"},
		Timeout:       "30s",
	})
	if !result.Allowed || len(result.Warnings) != 0 {
		t.Fatalf("built-in catalog without override should stay warning-free: %+v", result)
	}
}

func TestPolicyOverridesInvalidJSONWarnsAndUsesBuiltins(t *testing.T) {
	repoRoot := t.TempDir()
	agentHarnessDir := repoRoot + "/.agent-harness"
	if err := os.MkdirAll(agentHarnessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentHarnessDir+"/policy.json", []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: repoRoot,
		CWD:           repoRoot,
		Argv:          []string{"git", "status", "--short"},
		Timeout:       "30s",
	})
	if !result.Allowed {
		t.Fatalf("invalid override should not break built-in policy: %+v", result)
	}
	if !containsString(result.Warnings, "policy_override_parse_failed") {
		t.Fatalf("invalid override parse warning missing: %+v", result)
	}
}

func TestPolicyOverrideAffectsEvaluation(t *testing.T) {
	repoRoot := t.TempDir()
	agentHarnessDir := repoRoot + "/.agent-harness"
	if err := os.MkdirAll(agentHarnessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	policyJSON := `{"additional_read_only_commands": ["my-readonly-tool"]}`
	if err := os.WriteFile(agentHarnessDir+"/policy.json", []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	result := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: repoRoot,
		CWD:           repoRoot,
		Argv:          []string{"my-readonly-tool", "arg"},
		Timeout:       "30s",
	})
	if !result.Allowed {
		t.Fatalf("expected my-readonly-tool to be allowed with override: %+v", result)
	}
}

func TestPolicyOverridesDoNotLeakAcrossWorkspaceRoots(t *testing.T) {
	rootWithOverride := t.TempDir()
	agentHarnessDir := rootWithOverride + "/.agent-harness"
	if err := os.MkdirAll(agentHarnessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentHarnessDir+"/policy.json", []byte(`{"additional_read_only_commands":["repo-tool"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rootWithoutOverride := t.TempDir()

	allowed := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: rootWithOverride,
		CWD:           rootWithOverride,
		Argv:          []string{"repo-tool"},
		Timeout:       "30s",
	})
	if !allowed.Allowed {
		t.Fatalf("root with override should allow repo-tool: %+v", allowed)
	}

	denied := EvaluateCommandPolicy(CommandPolicyRequest{
		WorkspaceRoot: rootWithoutOverride,
		CWD:           rootWithoutOverride,
		Argv:          []string{"repo-tool"},
		Timeout:       "30s",
	})
	if denied.Allowed || !containsString(denied.DenyReasons, "command_not_in_read_only_allowlist") {
		t.Fatalf("override leaked into another root: %+v", denied)
	}
}

func TestPolicyTierClassifiesEveryFlagCombination(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		write    bool
		network  bool
		shell    bool
		wantName string
		wantCaps []string
	}{
		{false, false, false, PolicyTierReadOnly, []string{}},
		{true, false, false, PolicyTierWorkspaceWrite, []string{"write"}},
		{false, true, false, PolicyTierNetworkAccess, []string{"network"}},
		{true, true, false, PolicyTierNetworkAccess, []string{"network", "write"}},
		{false, false, true, PolicyTierShellException, []string{"shell"}},
		{true, false, true, PolicyTierShellException, []string{"shell", "write"}},
		{false, true, true, PolicyTierShellException, []string{"network", "shell"}},
		{true, true, true, PolicyTierShellException, []string{"network", "shell", "write"}},
	}
	for _, tc := range cases {
		result := EvaluateCommandPolicy(CommandPolicyRequest{
			WorkspaceRoot:  root,
			CWD:            root,
			Argv:           []string{"git", "status", "--short"},
			Timeout:        "30s",
			WriteAllowed:   tc.write,
			NetworkAllowed: tc.network,
			ShellAllowed:   tc.shell,
			ShellReason:    "test",
		})
		if result.Tier.Name != tc.wantName {
			t.Fatalf("write=%v network=%v shell=%v: want tier %q, got %q", tc.write, tc.network, tc.shell, tc.wantName, result.Tier.Name)
		}
		if strings.Join(result.Tier.GrantedCapabilities, ",") != strings.Join(tc.wantCaps, ",") {
			t.Fatalf("write=%v network=%v shell=%v: want caps %v, got %v", tc.write, tc.network, tc.shell, tc.wantCaps, result.Tier.GrantedCapabilities)
		}
		if result.Tier.Rationale == "" {
			t.Fatalf("tier %q must carry a rationale", tc.wantName)
		}
	}
}
