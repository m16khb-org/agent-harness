package nativeintegration

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

func nativeIntegrationRequiredPaths(root, home string, codexSkills, claudeSkills []string) []string {
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
	}
	for _, nativeSkill := range codexSkills {
		paths = append(paths, filepath.Join(home, ".codex", "skills", nativeSkill, "SKILL.md"))
	}
	for _, nativeSkill := range claudeSkills {
		paths = append(paths, filepath.Join(home, ".claude", "skills", nativeSkill, "SKILL.md"))
	}
	return paths
}

func nativeIntegrationPathErrors(paths []string, deps nativeIntegrationValidationDeps) []string {
	errs := []string{}
	for _, path := range paths {
		if !deps.exists(path) {
			errs = append(errs, "missing "+path)
		}
	}
	return errs
}

func nativeIntegrationCodexConfigErrors(home string, deps nativeIntegrationValidationDeps) []string {
	errs := []string{}
	if b, err := deps.readFile(filepath.Join(home, ".codex", "config.toml")); err != nil || !strings.Contains(string(b), "[mcp_servers.agent_harness]") {
		errs = append(errs, "Codex MCP config missing agent_harness")
	}
	if b, err := deps.readFile(filepath.Join(home, ".codex", "hooks.json")); err != nil || !strings.Contains(string(b), "hook user-prompt") {
		errs = append(errs, "Codex UserPromptSubmit hook missing agent-harness hook user-prompt")
	}
	return errs
}

func nativeIntegrationDuplicateWarningOutput(fixture string) ([]string, string) {
	duplicateWarnings := detectClaudeMCPDuplicateWarnings(fixture)
	warningBytes, _ := json.MarshalIndent(map[string]any{
		"duplicate_mcp_warning_fixture": duplicateWarnings,
	}, "", "  ")
	errs := []string{}
	if len(duplicateWarnings) != 1 || duplicateWarnings[0].Server != "agent_harness" || !strings.Contains(duplicateWarnings[0].Message, "multiple scopes") {
		errs = append(errs, "Claude duplicate MCP warning fixture was not classified")
	}
	return errs, string(warningBytes)
}
