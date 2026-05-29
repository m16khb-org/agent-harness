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

	for _, skillName := range req.SkillNames {
		userLink, err := installutil.EnsureSymlinkPlan(filepath.Join(req.Root, "skills", skillName), filepath.Join(req.Home, ".claude", "skills", skillName), req.DryRun)
		result.Links = append(result.Links, userLink)
		if err != nil {
			errs = append(errs, err)
		}
	}

	mcpConfig := claudeProjectMCPConfig()
	file, err := installutil.WriteJSONPlan(filepath.Join(req.Root, "configs", "claude", "mcp.project.json"), "claude_project_mcp_template", mcpConfig, 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	if req.ProjectLocal {
		for _, skillName := range req.SkillNames {
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
		result.Messages = append(result.Messages, "dry-run: planned Claude user/global and optional project-local files without writing")
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
			"agent_harness": map[string]any{
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
