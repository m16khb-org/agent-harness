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

	enabled, skipped := installutil.SkillNamesForHost(req.Root, req.SkillNames, "reasonix")
	for _, s := range skipped {
		result.Messages = append(result.Messages, "skip skill for reasonix: "+s)
	}

	// Try one write to the home directory. If the sandbox blocks it, skip
	// all home-dir writes gracefully rather than failing the entire install.
	homeWriteable := req.DryRun || canWriteTo(req.ReasonixHome)

	if homeWriteable {
		links, linkErrs := installutil.PlanSkillLinks(req.Root, filepath.Join(req.ReasonixHome, "skills"), enabled, req.DryRun)
		result.Links = append(result.Links, links...)
		errs = append(errs, linkErrs...)

		settingsPath := filepath.Join(req.ReasonixHome, "settings.json")
		file, err := writeReasonixSettings(settingsPath, req)
		result.Files = append(result.Files, file)
		if err != nil {
			errs = append(errs, err)
		}

		if req.ProjectLocal {
			for _, skillName := range enabled {
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
		result.Messages = append(result.Messages, "reasonix home directory ~/.reasonix not writable under current sandbox; skipping skill links and hook settings")
		result.Messages = append(result.Messages, "run `./bin/agent-harness update` in a regular terminal to enable Reasonix integration")
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

// canWriteTo reports whether a directory can be used for home-dir writes. It
// first ensures the directory exists (creating it when missing, as on a fresh
// install where ~/.reasonix has never been created), then probes with a temp
// file. Returning false when either step fails lets the installer skip
// home-dir writes gracefully under a sandbox rather than failing the whole
// install, while still succeeding on a normal first-time install.
func canWriteTo(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".harness-write-test-*")
	if err != nil {
		return false
	}
	f.Close()
	_ = os.Remove(f.Name())
	return true
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
