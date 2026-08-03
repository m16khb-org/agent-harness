package claude

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "claude" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installutil.NewPlan("claude", req.DryRun)

	enabledSkills, links, messages, skillErrs := installutil.PlanHostSkillLinks(req.Root, filepath.Join(req.Home, ".claude", "skills"), req.SkillNames, "claude", req.DryRun)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	settingsFile, hookMessages, settingsErr := writeClaudeSettings(filepath.Join(req.Home, ".claude", "settings.json"), req)
	plan.File(settingsFile, settingsErr)
	plan.Messages(hookMessages)
	plan.File(writeClaudeUserMCP(filepath.Join(req.Home, ".claude.json"), req))

	mcpConfig := claudeProjectMCPConfig()
	plan.File(installutil.WriteJSONPlan(filepath.Join(req.Root, "configs", "claude", "mcp.project.json"), "claude_project_mcp_template", mcpConfig, 0o644, req.DryRun))

	hooksTemplatePath := filepath.Join(req.Root, "configs", "claude", "hooks.settings.json")
	plan.File(installutil.WriteJSONPlan(hooksTemplatePath, "claude_hooks_template", claudeSettingsConfig("./bin/agent-harness"), 0o644, req.DryRun))

	if req.ProjectLocal {
		for _, skillName := range enabledSkills {
			plan.Link(installutil.EnsureSymlinkPlan(filepath.ToSlash(filepath.Join("..", "..", "skills", skillName)), filepath.Join(req.Root, ".claude", "skills", skillName), req.DryRun))
		}
		plan.File(installutil.WriteJSONPlan(filepath.Join(req.Root, ".mcp.json"), "claude_project_mcp_config", mcpConfig, 0o644, req.DryRun))
	}

	if req.DryRun {
		plan.Message("dry-run: planned Claude user skills, MCP config, and lifecycle hooks without writing")
	}

	return plan.Finish()
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
