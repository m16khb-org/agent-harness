package nativeintegration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateNativeIntegrationWithDepsCoversSuccessAndMissingPaths(t *testing.T) {
	root := t.TempDir()
	stubStableNativeRoot(t, func(string) (string, error) { return root, nil })
	home := t.TempDir()
	existing := nativeIntegrationExpectedPaths(root, home)
	deps := nativeIntegrationValidationDeps{
		userHomeDir: func() (string, error) { return home, nil },
		listSkills: func(string) ([]string, error) {
			return []string{"shared", "codex-only", "claude-only", "omo-only"}, nil
		},
		skillNamesForHost: func(_ string, _ []string, host string) ([]string, []string) {
			switch host {
			case "codex":
				return []string{"shared", "codex-only"}, nil
			case "claude":
				return []string{"shared", "claude-only"}, nil
			case "omo":
				return []string{"shared", "omo-only"}, nil
			default:
				return nil, nil
			}
		},
		exists: func(path string) bool { return existing[path] },
		readFile: func(path string) ([]byte, error) {
			switch {
			case path == filepath.Join(home, ".omo", "mcp.json"):
				return []byte(fmt.Sprintf(`{"mcpServers":{"agent_harness":{"command":%q,"args":["mcp"],"env":{"HARNESS_ROOT":%q}}}}`, filepath.Join(root, "bin", "agent-harness"), root)), nil
			case path == filepath.Join(home, ".omo", "extensions", "agent-harness.js"):
				return []byte(OmoLifecycleExtension(filepath.Join(root, "bin", "agent-harness"))), nil
			}
			switch filepath.Base(path) {
			case "config.toml":
				return []byte("[mcp_servers.agent_harness]\ncommand = \"agent-harness\"\n"), nil
			case "hooks.json":
				return []byte(fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"command":"'%s' hook session-start --host codex","timeout":5,"type":"command"}]}]}}`, filepath.Join(root, "bin", "agent-harness"))), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		duplicateWarningFixture: claudeMCPDuplicateWarningFixture,
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if !step.OK || step.Label != "native integration" || !strings.Contains(step.Stdout, "duplicate_mcp_warning_fixture") {
		t.Fatalf("unexpected success step: %#v", step)
	}

	missingPath := filepath.Join(home, ".omo", "agent", "skills", "omo-only", "SKILL.md")
	existing[missingPath] = false
	failed := validateNativeIntegrationWithDeps(root, deps)
	if failed.OK || !strings.Contains(failed.Error, "missing "+missingPath) {
		t.Fatalf("expected missing path failure, got %#v", failed)
	}
}

func TestValidateNativeIntegrationWithDepsCoversSkillConfigAndWarningFailures(t *testing.T) {
	root := t.TempDir()
	stubStableNativeRoot(t, func(string) (string, error) { return root, nil })
	home := t.TempDir()
	existing := nativeIntegrationExpectedPaths(root, home)
	deps := nativeIntegrationValidationDeps{
		userHomeDir: func() (string, error) { return home, nil },
		listSkills:  func(string) ([]string, error) { return nil, errors.New("skill list failed") },
		skillNamesForHost: func(string, []string, string) ([]string, []string) {
			return nil, nil
		},
		exists: func(path string) bool { return existing[path] },
		readFile: func(path string) ([]byte, error) {
			if strings.HasSuffix(path, "config.toml") {
				return []byte("[mcp_servers.other]\n"), nil
			}
			return []byte(`{"command":"other hook"}`), nil
		},
		duplicateWarningFixture: func() string { return "agent_harness: ./bin/agent-harness mcp - Connected\n" },
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if step.OK {
		t.Fatalf("expected aggregate failure, got %#v", step)
	}
	for _, want := range []string{
		"list native skills: skill list failed",
		"Codex MCP config missing agent_harness",
		"Codex thin context hooks missing agent-harness SessionStart surface",
		"Omo MCP config missing canonical agent_harness server",
		"Omo lifecycle extension missing canonical session_start/session_compact surface",
		"Claude duplicate MCP warning fixture was not classified",
	} {
		if !strings.Contains(step.Error, want) {
			t.Fatalf("expected %q in error, got %#v", want, step)
		}
	}
}

func TestValidateNativeIntegrationReportsStableRootResolutionError(t *testing.T) {
	root := t.TempDir()
	stubStableNativeRoot(t, func(string) (string, error) { return "", errors.New("stable root unavailable") })
	home := t.TempDir()
	existing := nativeIntegrationExpectedPaths(root, home)
	deps := nativeIntegrationValidationDeps{
		userHomeDir: func() (string, error) { return home, nil },
		listSkills:  func(string) ([]string, error) { return []string{"shared", "codex-only", "claude-only"}, nil },
		skillNamesForHost: func(_ string, _ []string, host string) ([]string, []string) {
			if host == "codex" {
				return []string{"shared", "codex-only"}, nil
			}
			return []string{"shared", "claude-only"}, nil
		},
		exists: func(path string) bool { return existing[path] },
		readFile: func(path string) ([]byte, error) {
			switch filepath.Base(path) {
			case "config.toml":
				return []byte("[mcp_servers.agent_harness]\n"), nil
			case "hooks.json":
				return []byte(fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"command":"'%s' hook session-start --host codex","timeout":5,"type":"command"}]}]}}`, filepath.Join(root, "bin", "agent-harness"))), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		duplicateWarningFixture: claudeMCPDuplicateWarningFixture,
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if step.OK || !strings.Contains(step.Error, "resolve stable native root: stable root unavailable") {
		t.Fatalf("native integration must fail closed on stable-root resolution: %#v", step)
	}
}

func TestHasThinCodexContextHooksPermitsThirdPartyLifecycleEvents(t *testing.T) {
	config := `{
		"hooks": {
			"SessionStart": [
				{"hooks": [{"type": "command", "command": "'/Users/example/.orca/agent-hooks/codex-hook.sh' observe", "timeout": 10}]},
				{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook session-start --host codex", "timeout": 5}]}
			],
			"PreToolUse": [{"hooks": [{"type": "command", "command": "'/Users/example/.orca/agent-hooks/codex-hook.sh' observe", "timeout": 10}]}],
			"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "codegraph observe", "timeout": 10}]}],
			"SubagentStop": [{"hooks": [{"type": "command", "command": "third-party stop", "timeout": 10}]}],
			"PermissionRequest": [{"hooks": [{"type": "command", "command": "third-party permission", "timeout": 10}]}]
		}
	}`
	if !hasThinCodexContextHooks(config, "/source/bin/agent-harness") {
		t.Fatal("third-party lifecycle hooks must not invalidate the managed context hook")
	}
}

func TestHasThinCodexContextHooksRejectsLegacyManagedEvent(t *testing.T) {
	config := `{
		"hooks": {
			"SessionStart": [{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook session-start --host codex", "timeout": 5}]}],
			"PreToolUse": [{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook pre-tool-use --host codex --enforce-worktree", "timeout": 5}]}]
		}
	}`
	if hasThinCodexContextHooks(config, "/source/bin/agent-harness") {
		t.Fatal("legacy agent-harness enforcement event must invalidate the managed context-hook surface")
	}
}

func TestHasThinCodexContextHooksUsesCanonicalGroupsForManagedCommands(t *testing.T) {
	for name, config := range map[string]string{
		"quoted canonical path with spaces": `{
			"hooks": {
				"SessionStart": [{"hooks": [{"type": "command", "command": "'/source with spaces/bin/agent-harness' hook session-start --host codex", "timeout": 5}]}]
			}
		}`,
		"legacy no-host event": `{
			"hooks": {
				"SessionStart": [{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook session-start --host codex", "timeout": 5}]}],
				"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook user-prompt", "timeout": 5}]}]
			}
		}`,
		"wrong host alongside required hooks": `{
			"hooks": {
				"SessionStart": [
					{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook session-start --host codex", "timeout": 5}]},
					{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook session-start --host claude", "timeout": 5}]}
				]
			}
		}`,
		"extra argument alongside required hooks": `{
			"hooks": {
				"SessionStart": [
					{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook session-start --host codex", "timeout": 5}]},
					{"hooks": [{"type": "command", "command": "'/source/bin/agent-harness' hook session-start --host codex --legacy", "timeout": 5}]}
				]
			}
		}`,
		"wrong binary path": `{
			"hooks": {
				"SessionStart": [{"hooks": [{"type": "command", "command": "'/other/bin/agent-harness' hook session-start --host codex", "timeout": 5}]}]
			}
		}`,
		"malformed JSON": `{"hooks":`,
	} {
		t.Run(name, func(t *testing.T) {
			expectedBinary := "/source/bin/agent-harness"
			if name == "quoted canonical path with spaces" {
				expectedBinary = "/source with spaces/bin/agent-harness"
				if !hasThinCodexContextHooks(config, expectedBinary) {
					t.Fatal("quoted canonical path must validate")
				}
				return
			}
			if hasThinCodexContextHooks(config, expectedBinary) {
				t.Fatal("non-canonical managed hook config was accepted")
			}
		})
	}
}

func nativeIntegrationExpectedPaths(root, home string) map[string]bool {
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(root, "configs", "omo", "mcp.json"),
		filepath.Join(root, "configs", "omo", "agent-harness.js"),
		filepath.Join(home, ".codex", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "codex-only", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "claude-only", "SKILL.md"),
		filepath.Join(home, ".omo", "agent", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".omo", "agent", "skills", "omo-only", "SKILL.md"),
	}
	out := map[string]bool{}
	for _, path := range paths {
		out[path] = true
	}
	return out
}

func stubStableNativeRoot(t *testing.T, resolver func(string) (string, error)) {
	t.Helper()
	previous := ResolveStableNativeRoot
	ResolveStableNativeRoot = resolver
	t.Cleanup(func() { ResolveStableNativeRoot = previous })
}

func TestValidateNativeIntegrationWithDepsCoversHomeFailure(t *testing.T) {
	step := validateNativeIntegrationWithDeps(t.TempDir(), nativeIntegrationValidationDeps{
		userHomeDir: func() (string, error) { return "", os.ErrNotExist },
	})
	if step.OK || !strings.Contains(step.Error, "user home:") {
		t.Fatalf("expected user home failure, got %#v", step)
	}
}

func TestDetectClaudeMCPDuplicateWarnings(t *testing.T) {
	warnings := detectClaudeMCPDuplicateWarnings(claudeMCPDuplicateWarningFixture())
	if len(warnings) != 1 {
		t.Fatalf("expected one duplicate warning, got %+v", warnings)
	}
	if warnings[0].Server != "agent_harness" || !strings.Contains(warnings[0].Message, "multiple scopes") {
		t.Fatalf("duplicate warning was not classified: %+v", warnings[0])
	}
	if len(warnings[0].Suggestions) != 1 || !strings.Contains(warnings[0].Suggestions[0], "claude mcp remove agent_harness") {
		t.Fatalf("duplicate warning suggestion missing: %+v", warnings[0].Suggestions)
	}
	if got := detectClaudeMCPDuplicateWarnings("agent_harness: ./bin/agent-harness mcp - ✓ Connected\n"); len(got) != 0 {
		t.Fatalf("non-conflicting output produced warnings: %+v", got)
	}
}
