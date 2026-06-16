package codex

import (
	"path/filepath"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "codex" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installutil.NewPlan("codex", req.DryRun)

	_, links, messages, skillErrs := installutil.PlanHostSkillLinks(req.Root, filepath.Join(req.CodexHome, "skills"), req.SkillNames, "codex", req.DryRun)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	plan.File(writeGlobalConfig(filepath.Join(req.CodexHome, "config.toml"), req))

	mcpTemplatePath := filepath.Join(req.Root, "configs", "codex", "mcp.config.toml")
	plan.File(installutil.WriteTextPlan(mcpTemplatePath, "codex_mcp_template", codexTemplate(req), 0o644, req.DryRun))

	plan.File(writeCodexHooks(filepath.Join(req.CodexHome, "hooks.json"), req))

	hooksTemplatePath := filepath.Join(req.Root, "configs", "codex", "hooks.json")
	plan.File(installutil.WriteJSONPlan(hooksTemplatePath, "codex_hooks_template", codexHooksConfig("./bin/agent-harness"), 0o644, req.DryRun))

	patchedFiles, patchMessages, err := patchCodexPluginHookCompatibility(req)
	plan.Files(patchedFiles)
	plan.Messages(patchMessages)
	plan.Err(err)

	if req.DryRun {
		plan.Message("dry-run: planned Codex user skill links, MCP config, and lifecycle hooks without writing")
	}

	return plan.Finish()
}
