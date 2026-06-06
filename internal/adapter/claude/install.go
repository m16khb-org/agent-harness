package claude

import (
	"errors"
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
