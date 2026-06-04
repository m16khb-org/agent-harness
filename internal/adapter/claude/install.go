package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "claude" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	result := port.HostInstallResult{Host: "claude", OK: true, DryRun: req.DryRun}
	var errs []error

	enabledSkills, links, messages, skillErrs := installutil.PlanHostSkillLinks(req.Root, filepath.Join(req.Home, ".claude", "skills"), req.SkillNames, "claude", req.DryRun)
	result.Messages = append(result.Messages, messages...)
	result.Links = append(result.Links, links...)
	errs = append(errs, skillErrs...)

	settingsPath := filepath.Join(req.Home, ".claude", "settings.json")
	file, err := writeClaudeSettings(settingsPath, req)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	mcpConfig := claudeProjectMCPConfig()
	file, err = installutil.WriteJSONPlan(filepath.Join(req.Root, "configs", "claude", "mcp.project.json"), "claude_project_mcp_template", mcpConfig, 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	hooksTemplatePath := filepath.Join(req.Root, "configs", "claude", "hooks.settings.json")
	file, err = installutil.WriteJSONPlan(hooksTemplatePath, "claude_hooks_template", claudeSettingsConfig("./bin/agent-harness"), 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	if req.ProjectLocal {
		for _, skillName := range enabledSkills {
			projectLink, err := installutil.EnsureSymlinkPlan(filepath.ToSlash(filepath.Join("..", "..", "skills", skillName)), filepath.Join(req.Root, ".claude", "skills", skillName), req.DryRun)
			result.Links = append(result.Links, projectLink)
			if err != nil {
				errs = append(errs, err)
			}
		}
		file, err = installutil.WriteJSONPlan(filepath.Join(req.Root, ".mcp.json"), "claude_project_mcp_config", mcpConfig, 0o644, req.DryRun)
		result.Files = append(result.Files, file)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if req.DryRun {
		result.Messages = append(result.Messages, "dry-run: planned Claude user skills, MCP config, and lifecycle hooks without writing")
	}

	if len(errs) > 0 {
		result.OK = false
		return result, joinErrors(errs)
	}
	return result, nil
}

func claudeProjectMCPConfig() map[string]any {
	return map[string]any{
		"mcpServers": map[string]any{
			"agent_harness_project": map[string]any{
				"type":    "stdio",
				"command": "./bin/agent-harness",
				"args":    []string{"mcp"},
				"env": map[string]any{
					"HARNESS_ROOT": ".",
				},
			},
		},
	}
}

func writeClaudeSettings(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "claude_user_settings"}
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return file, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	return installutil.WriteJSONPlan(path, file.Kind, mergeClaudeHookConfig(config, req.BinPath), 0o644, req.DryRun)
}

func claudeSettingsConfig(binPath string) map[string]any {
	return mergeClaudeHookConfig(map[string]any{}, binPath)
}

func mergeClaudeHookConfig(config map[string]any, binPath string) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	for _, spec := range claudeLifecycleHookSpecs(binPath) {
		groups := []any{}
		if existing, ok := hooks[spec.Event].([]any); ok {
			for _, group := range existing {
				if !claudeHookGroupContainsAgentHarness(group) {
					groups = append(groups, group)
				}
			}
		}
		groups = append(groups, claudeHookGroup(spec))
		hooks[spec.Event] = groups
	}
	return config
}

type claudeLifecycleHookSpec struct {
	BinPath    string
	Event      string
	Subcommand string
	Matcher    string
	Timeout    int
}

func claudeLifecycleHookSpecs(binPath string) []claudeLifecycleHookSpec {
	return []claudeLifecycleHookSpec{
		{BinPath: binPath, Event: "SessionStart", Subcommand: "session-start", Timeout: 5},
		{BinPath: binPath, Event: "UserPromptSubmit", Subcommand: "user-prompt", Timeout: 5},
		{BinPath: binPath, Event: "PreToolUse", Subcommand: "pre-tool-use", Matcher: "*", Timeout: 5},
		{BinPath: binPath, Event: "PostToolUse", Subcommand: "post-tool-use", Matcher: "*", Timeout: 5},
		{BinPath: binPath, Event: "PreCompact", Subcommand: "pre-compact", Timeout: 5},
		{BinPath: binPath, Event: "PostCompact", Subcommand: "post-compact", Timeout: 5},
		{BinPath: binPath, Event: "Stop", Subcommand: "stop", Timeout: 5},
	}
}

func claudeHookGroup(spec claudeLifecycleHookSpec) map[string]any {
	group := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": claudeHookCommand(spec.BinPath, spec.Subcommand),
				"timeout": spec.Timeout,
			},
		},
	}
	if spec.Matcher != "" {
		group["matcher"] = spec.Matcher
	}
	return group
}

func claudeHookCommand(binPath, subcommand string) string {
	cmd := fmt.Sprintf("%s hook %s", shellQuote(binPath), subcommand)
	if subcommand == "pre-tool-use" {
		cmd += " --host claude --enforce-worktree --enforce-korean-remote-artifacts --enforce-vcs-issue-linking --enforce-staged-checks --enforce-gitops-kubectl"
	}
	if subcommand == "stop" {
		cmd += " --host claude --enforce-numbered-next-actions --auto-proceed-next-actions"
	}
	return cmd
}

func claudeHookGroupContainsAgentHarness(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "harness") && strings.Contains(cmd, " hook ") {
			return true
		}
	}
	return false
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func joinErrors(errs []error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
