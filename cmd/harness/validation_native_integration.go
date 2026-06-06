package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/core"
)

type nativeIntegrationValidationDeps struct {
	userHomeDir             func() (string, error)
	listSkills              func(string) ([]string, error)
	skillNamesForHost       func(string, []string, string) ([]string, []string)
	exists                  func(string) bool
	readFile                func(string) ([]byte, error)
	duplicateWarningFixture func() string
}

func (deps nativeIntegrationValidationDeps) withDefaults() nativeIntegrationValidationDeps {
	if deps.userHomeDir == nil {
		deps.userHomeDir = os.UserHomeDir
	}
	if deps.listSkills == nil {
		deps.listSkills = core.ListSkillNames
	}
	if deps.skillNamesForHost == nil {
		deps.skillNamesForHost = installutil.SkillNamesForHost
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.readFile == nil {
		deps.readFile = os.ReadFile
	}
	if deps.duplicateWarningFixture == nil {
		deps.duplicateWarningFixture = claudeMCPDuplicateWarningFixture
	}
	return deps
}

func validateNativeIntegration(root string) StepResult {
	return validateNativeIntegrationWithDeps(root, nativeIntegrationValidationDeps{})
}

func validateNativeIntegrationWithDeps(root string, deps nativeIntegrationValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	home, err := deps.userHomeDir()
	if err != nil {
		return failedStep("native integration", fmt.Errorf("user home: %w", err))
	}
	errs := []string{}
	stdoutParts := []string{}
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
	}
	nativeSkills, err := deps.listSkills(root)
	if err != nil {
		errs = append(errs, "list native skills: "+err.Error())
	}
	codexSkills, _ := deps.skillNamesForHost(root, nativeSkills, "codex")
	claudeSkills, _ := deps.skillNamesForHost(root, nativeSkills, "claude")
	for _, nativeSkill := range codexSkills {
		paths = append(paths, filepath.Join(home, ".codex", "skills", nativeSkill, "SKILL.md"))
	}
	for _, nativeSkill := range claudeSkills {
		paths = append(paths, filepath.Join(home, ".claude", "skills", nativeSkill, "SKILL.md"))
	}
	for _, path := range paths {
		if !deps.exists(path) {
			errs = append(errs, "missing "+path)
		}
	}
	if b, err := deps.readFile(filepath.Join(home, ".codex", "config.toml")); err != nil || !strings.Contains(string(b), "[mcp_servers.agent_harness]") {
		errs = append(errs, "Codex MCP config missing agent_harness")
	}
	if b, err := deps.readFile(filepath.Join(home, ".codex", "hooks.json")); err != nil || !strings.Contains(string(b), "hook user-prompt") {
		errs = append(errs, "Codex UserPromptSubmit hook missing agent-harness hook user-prompt")
	}
	duplicateWarnings := detectClaudeMCPDuplicateWarnings(deps.duplicateWarningFixture())
	warningBytes, _ := json.MarshalIndent(map[string]any{
		"duplicate_mcp_warning_fixture": duplicateWarnings,
	}, "", "  ")
	stdoutParts = append(stdoutParts, string(warningBytes))
	if len(duplicateWarnings) != 1 || duplicateWarnings[0].Server != "agent_harness" || !strings.Contains(duplicateWarnings[0].Message, "multiple scopes") {
		errs = append(errs, "Claude duplicate MCP warning fixture was not classified")
	}
	if len(errs) > 0 {
		return assertionStepWithOutput("native integration", started, errs, stdoutParts, nil)
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "native integration",
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
