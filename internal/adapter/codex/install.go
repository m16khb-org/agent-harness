package codex

import (
	"path/filepath"

	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "codex" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := NewInstallPlan("codex", req.DryRun)

	_, links, messages, skillErrs := PlanHostSkillLinks(req.Root, filepath.Join(req.CodexHome, "skills"), req.SkillNames, "codex", req.DryRun)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	plan.File(writeGlobalConfig(filepath.Join(req.CodexHome, "config.toml"), req))

	mcpTemplatePath := filepath.Join(req.Root, "configs", "codex", "mcp.config.toml")
	plan.File(WriteTextPlan(mcpTemplatePath, "codex_mcp_template", codexTemplate(req), 0o644, req.DryRun))

	hooksFile, hookMessages, hooksErr := writeCodexHooks(filepath.Join(req.CodexHome, "hooks.json"), req)
	plan.File(hooksFile, hooksErr)
	plan.Messages(hookMessages)

	hooksTemplatePath := filepath.Join(req.Root, "configs", "codex", "hooks.json")
	plan.File(WriteJSONPlan(hooksTemplatePath, "codex_hooks_template", codexHooksConfig("./bin/agent-harness"), 0o644, req.DryRun))

	if req.DryRun {
		plan.Message("dry-run: planned Codex user skill links, MCP config, and lifecycle hooks without writing")
	}

	return plan.Finish()
}
