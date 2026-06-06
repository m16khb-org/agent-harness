package nativeintegration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateNativeIntegrationWithDepsCoversSuccessAndMissingPaths(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	existing := nativeIntegrationExpectedPaths(root, home)
	deps := nativeIntegrationValidationDeps{
		userHomeDir: func() (string, error) { return home, nil },
		listSkills:  func(string) ([]string, error) { return []string{"shared", "codex-only", "claude-only"}, nil },
		skillNamesForHost: func(_ string, _ []string, host string) ([]string, []string) {
			switch host {
			case "codex":
				return []string{"shared", "codex-only"}, nil
			case "claude":
				return []string{"shared", "claude-only"}, nil
			default:
				return nil, nil
			}
		},
		exists: func(path string) bool { return existing[path] },
		readFile: func(path string) ([]byte, error) {
			switch filepath.Base(path) {
			case "config.toml":
				return []byte("[mcp_servers.agent_harness]\ncommand = \"agent-harness\"\n"), nil
			case "hooks.json":
				return []byte(`{"command":"agent-harness hook user-prompt"}`), nil
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

	missingPath := filepath.Join(home, ".claude", "skills", "claude-only", "SKILL.md")
	existing[missingPath] = false
	failed := validateNativeIntegrationWithDeps(root, deps)
	if failed.OK || !strings.Contains(failed.Error, "missing "+missingPath) {
		t.Fatalf("expected missing path failure, got %#v", failed)
	}
}

func TestValidateNativeIntegrationWithDepsCoversSkillConfigAndWarningFailures(t *testing.T) {
	root := t.TempDir()
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
		"Codex UserPromptSubmit hook missing agent-harness hook user-prompt",
		"Claude duplicate MCP warning fixture was not classified",
	} {
		if !strings.Contains(step.Error, want) {
			t.Fatalf("expected %q in error, got %#v", want, step)
		}
	}
}

func nativeIntegrationExpectedPaths(root, home string) map[string]bool {
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(home, ".codex", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "codex-only", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "claude-only", "SKILL.md"),
	}
	out := map[string]bool{}
	for _, path := range paths {
		out[path] = true
	}
	return out
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
