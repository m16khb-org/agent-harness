package reasonix

import (
	"os"
	"path/filepath"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "reasonix" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installutil.NewPlan("reasonix", req.DryRun)

	enabled, skipped := installutil.SkillNamesForHost(req.Root, req.SkillNames, "reasonix")
	for _, s := range skipped {
		plan.Message("skip skill for reasonix: " + s)
	}

	// Try one write to the home directory. If the sandbox blocks it, skip
	// all home-dir writes gracefully rather than failing the entire install.
	homeWriteable := req.DryRun || canWriteTo(req.ReasonixHome)

	if homeWriteable {
		links, linkErrs := installutil.PlanSkillLinks(req.Root, filepath.Join(req.ReasonixHome, "skills"), enabled, req.DryRun)
		plan.Links(links)
		plan.Errs(linkErrs)

		plan.File(writeReasonixSettings(filepath.Join(req.ReasonixHome, "settings.json"), req))

		if req.ProjectLocal {
			for _, skillName := range enabled {
				plan.Link(installutil.EnsureSymlinkPlan(filepath.ToSlash(filepath.Join("..", "..", "skills", skillName)), filepath.Join(req.Root, ".reasonix", "skills", skillName), req.DryRun))
			}
			plan.File(installutil.WriteJSONPlan(filepath.Join(req.Root, ".reasonix", "settings.json"), "reasonix_project_settings", reasonixSettingsConfig("./bin/agent-harness"), 0o644, req.DryRun))
		}
	} else {
		plan.Message("reasonix home directory ~/.reasonix not writable under current sandbox; skipping skill links and hook settings")
		plan.Message("run `./bin/agent-harness update` in a regular terminal to enable Reasonix integration")
	}

	plan.File(writeReasonixMCPConfig(req))

	mcpTemplate := reasonixProjectMCPTemplate()
	plan.File(installutil.WriteTextPlan(filepath.Join(req.Root, "configs", "reasonix", "mcp.config.toml"), "reasonix_project_mcp_template", mcpTemplate, 0o644, req.DryRun))

	hooksTemplatePath := filepath.Join(req.Root, "configs", "reasonix", "hooks.settings.json")
	plan.File(installutil.WriteJSONPlan(hooksTemplatePath, "reasonix_hooks_template", reasonixSettingsConfig("./bin/agent-harness"), 0o644, req.DryRun))

	if req.DryRun {
		plan.Message("dry-run: planned Reasonix user skills, MCP config, and lifecycle hooks without writing")
	}

	return plan.Finish()
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
