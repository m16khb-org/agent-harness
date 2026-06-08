package reasonix

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "reasonix" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	result := port.HostInstallResult{Host: "reasonix", OK: true, DryRun: req.DryRun}
	var errs []error

	reasonixHomeWriteable := req.DryRun || ensureDir(req.ReasonixHome)
	skillsDir := filepath.Join(req.ReasonixHome, "skills")
	skillsWriteable := reasonixHomeWriteable && (req.DryRun || ensureDir(skillsDir))

	if skillsWriteable {
		enabledSkills, links, messages, skillErrs := installutil.PlanHostSkillLinks(req.Root, skillsDir, req.SkillNames, "reasonix", req.DryRun)
		result.Messages = append(result.Messages, messages...)
		result.Links = append(result.Links, links...)
		errs = append(errs, skillErrs...)

		if req.ProjectLocal {
			for _, skillName := range enabledSkills {
				projectLink, err := installutil.EnsureSymlinkPlan(filepath.ToSlash(filepath.Join("..", "..", "skills", skillName)), filepath.Join(req.Root, ".reasonix", "skills", skillName), req.DryRun)
				result.Links = append(result.Links, projectLink)
				if err != nil {
					errs = append(errs, err)
				}
			}
			file, err := installutil.WriteJSONPlan(filepath.Join(req.Root, ".reasonix", "settings.json"), "reasonix_project_settings", reasonixSettingsConfig("./bin/agent-harness"), 0o644, req.DryRun)
			result.Files = append(result.Files, file)
			if err != nil {
				errs = append(errs, err)
			}
		}
	} else {
		result.Messages = append(result.Messages, "reasonix home directory ~/.reasonix not writable; skipping skill links")
		result.Messages = append(result.Messages, "run `mkdir -p ~/.reasonix/skills` in a regular terminal to enable Reasonix integration")
	}

	if reasonixHomeWriteable {
		settingsPath := filepath.Join(req.ReasonixHome, "settings.json")
		file, err := writeReasonixSettings(settingsPath, req)
		result.Files = append(result.Files, file)
		if err != nil {
			errs = append(errs, err)
		}
	} else {
		result.Messages = append(result.Messages, "reasonix home directory not writable; skipping hook settings")
	}

	mcpFile, err := writeReasonixMCPConfig(req)
	result.Files = append(result.Files, mcpFile)
	if err != nil {
		errs = append(errs, err)
	}

	mcpTemplate := reasonixProjectMCPTemplate()
	file, err := installutil.WriteTextPlan(filepath.Join(req.Root, "configs", "reasonix", "mcp.config.toml"), "reasonix_project_mcp_template", mcpTemplate, 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	hooksTemplatePath := filepath.Join(req.Root, "configs", "reasonix", "hooks.settings.json")
	file, err = installutil.WriteJSONPlan(hooksTemplatePath, "reasonix_hooks_template", reasonixSettingsConfig("./bin/agent-harness"), 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	if req.DryRun {
		result.Messages = append(result.Messages, "dry-run: planned Reasonix user skills, MCP config, and lifecycle hooks without writing")
	}

	if len(errs) > 0 {
		result.OK = false
		return result, joinErrors(errs)
	}
	return result, nil
}

func reasonixProjectMCPTemplate() string {
	return `[[plugins]]
name = "agent_harness_project"
command = "./bin/agent-harness"
args = ["mcp"]
`
}

func ensureDir(path string) bool {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return true
	}
	return os.MkdirAll(path, 0o755) == nil
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
