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
	result := port.HostInstallResult{Host: "codex", OK: true, DryRun: req.DryRun}
	var errs []error

	_, links, messages, skillErrs := installutil.PlanHostSkillLinks(req.Root, filepath.Join(req.CodexHome, "skills"), req.SkillNames, "codex", req.DryRun)
	result.Messages = append(result.Messages, messages...)
	result.Links = append(result.Links, links...)
	errs = append(errs, skillErrs...)

	globalConfig := filepath.Join(req.CodexHome, "config.toml")
	file, err := writeGlobalConfig(globalConfig, req)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	templatePath := filepath.Join(req.Root, "configs", "codex", "mcp.config.toml")
	file, err = installutil.WriteTextPlan(templatePath, "codex_mcp_template", codexTemplate(req), 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	hooksPath := filepath.Join(req.CodexHome, "hooks.json")
	file, err = writeCodexHooks(hooksPath, req)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	hooksTemplatePath := filepath.Join(req.Root, "configs", "codex", "hooks.json")
	file, err = installutil.WriteJSONPlan(hooksTemplatePath, "codex_hooks_template", codexHooksConfig("./bin/agent-harness"), 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	patchedFiles, patchMessages, err := patchCodexPluginHookCompatibility(req)
	result.Files = append(result.Files, patchedFiles...)
	result.Messages = append(result.Messages, patchMessages...)
	if err != nil {
		errs = append(errs, err)
	}

	if req.DryRun {
		result.Messages = append(result.Messages, "dry-run: planned Codex user skill links, MCP config, and lifecycle hooks without writing")
	}

	if len(errs) > 0 {
		result.OK = false
		return result, joinErrors(errs)
	}
	return result, nil
}
