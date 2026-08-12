package omo

import (
	"path/filepath"

	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "omo" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := NewInstallPlan("omo", req.DryRun)
	enabledSkills, links, messages, skillErrs := PlanHostSkillLinks(
		req.Root,
		filepath.Join(req.Home, ".omo", "skills"),
		req.SkillNames,
		"omo",
		req.DryRun,
	)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	omoRoot := filepath.Join(req.Home, ".omo")
	plan.File(writeOmoUserMCP(filepath.Join(omoRoot, "mcp.json"), req))
	plan.File(WriteTextPlan(
		filepath.Join(omoRoot, "extensions", "agent-harness.js"),
		"omo_user_lifecycle_extension",
		omoLifecycleExtension(req.BinPath),
		0o644,
		req.DryRun,
	))
	plan.File(writeOmoProjectMCP(
		filepath.Join(req.Root, "configs", "omo", "mcp.json"),
		"omo_project_mcp_template",
		req.DryRun,
	))
	plan.File(WriteTextPlan(
		filepath.Join(req.Root, "configs", "omo", "agent-harness.js"),
		"omo_lifecycle_extension_template",
		omoLifecycleExtension("./bin/agent-harness"),
		0o644,
		req.DryRun,
	))

	if req.ProjectLocal {
		for _, skillName := range enabledSkills {
			target := filepath.ToSlash(filepath.Join("..", "..", "skills", skillName))
			path := filepath.Join(req.Root, ".omo", "skills", skillName)
			plan.Link(EnsureSymlinkPlan(target, path, req.DryRun))
		}
		plan.File(writeOmoProjectMCP(
			filepath.Join(req.Root, ".omo", "mcp.json"),
			"omo_project_mcp_config",
			req.DryRun,
		))
	}

	if req.DryRun {
		plan.Message("dry-run: planned Omo native skills, MCP config, and lifecycle extension without writing")
	}
	return plan.Finish()
}
