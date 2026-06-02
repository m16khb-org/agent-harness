package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func patchCodexPluginHookCompatibility(req port.NativeInstallRequest) ([]port.InstallFile, []string, error) {
	patches := []hookCompatibilityPatch{
		{
			Kind: "codex_plugin_hook_compat_llm_wiki",
			Globs: []string{
				filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki-marketplace", "llm-wiki", "*", "hooks", "llm-wiki-hook.cjs"),
				filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki", "llm-wiki", "*", "hooks", "llm-wiki-hook.cjs"),
			},
			Replacements: []textReplacement{
				{Old: "    },\n    suppressOutput: true\n  }));", New: "    }\n  }));"},
			},
		},
		{
			Kind: "codex_plugin_hook_compat_claude_mem",
			Globs: []string{
				filepath.Join(req.CodexHome, "plugins", "cache", "*", "claude-mem", "*", "hooks", "codex-hooks.json"),
				filepath.Join(req.CodexHome, "plugins", "cache", "*", "claude-mem", "*", "scripts", "worker-service.cjs"),
				filepath.Join(req.CodexHome, "plugins", "cache", "*", "claude-mem", "*", "scripts", "worker-cli.js"),
			},
			Replacements: []textReplacement{
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" start`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" start || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex context`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex context || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex session-init`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex session-init || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex file-context`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex file-context || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex observation`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex observation || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex summarize`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex summarize || true`},
				{Old: "function fZ(t,e){return{continue:!0,suppressOutput:!0,status:t,...e&&{message:e}}}", New: "function fZ(t,e){return{continue:!0,...e&&{systemMessage:e}}}"},
				{Old: "{continue:!0,suppressOutput:!0}", New: "{continue:!0}"},
				{Old: ",suppressOutput:!0", New: ""},
				{Old: "suppressOutput:!0,", New: ""},
				{Old: `O='{"continue": true, "suppressOutput": true}'`, New: `O='{"continue": true}'`},
				{Old: `O='{"continue":true,"suppressOutput":true}'`, New: `O='{"continue":true}'`},
			},
		},
	}
	var files []port.InstallFile
	var messages []string
	var errs []error
	for _, patch := range patches {
		paths := expandPatchGlobs(patch.Globs)
		if len(paths) == 0 {
			continue
		}
		for _, path := range paths {
			file, changed, err := applyHookCompatibilityPatch(path, patch.Kind, patch.Replacements, req.DryRun)
			if file.Path != "" {
				files = append(files, file)
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if changed {
				if req.DryRun {
					messages = append(messages, "dry-run: would patch Codex plugin hook compatibility: "+path)
				} else {
					messages = append(messages, "patched Codex plugin hook compatibility: "+path)
				}
			}
		}
	}
	return files, messages, joinErrors(errs)
}

type hookCompatibilityPatch struct {
	Kind         string
	Globs        []string
	Replacements []textReplacement
}

type textReplacement struct {
	Old string
	New string
}

func expandPatchGlobs(globs []string) []string {
	seen := map[string]bool{}
	var paths []string
	for _, pattern := range globs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func applyHookCompatibilityPatch(path, kind string, replacements []textReplacement, dryRun bool) (port.InstallFile, bool, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	b, err := os.ReadFile(path)
	if err != nil {
		return file, false, err
	}
	text := string(b)
	next := text
	for _, replacement := range replacements {
		next = strings.ReplaceAll(next, replacement.Old, replacement.New)
	}
	if next == text {
		return file, false, nil
	}
	if dryRun {
		file.WouldWrite = true
		return file, true, nil
	}
	backup := path + ".harness.bak"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		if err := os.WriteFile(backup, b, 0o600); err != nil {
			return file, false, err
		}
	} else if err != nil {
		return file, false, err
	}
	if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
		return file, false, err
	}
	file.Written = true
	return file, true, nil
}

func writeCodexHooks(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "codex_user_hooks_config"}
	config := map[string]any{}
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &config); err != nil {
			return file, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	merged := mergeHookConfig(config, req.BinPath)
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return file, err
	}
	text := string(append(b, '\n'))
	if existing, err := os.ReadFile(path); err == nil && string(existing) == text {
		return file, nil
	}
	if req.DryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func writeGlobalConfig(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "codex_user_mcp_config"}
	text := ""
	if b, err := os.ReadFile(path); err == nil {
		text = string(b)
		if !req.DryRun {
			backup := path + ".harness.bak"
			if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
				if writeErr := os.WriteFile(backup, []byte(text), 0o600); writeErr != nil {
					return file, writeErr
				}
			}
		}
	} else if !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	for _, section := range []string{"mcp_servers.agent_harness", "mcp_servers.agent_harness.env", "mcp_servers.agent-harness", "mcp_servers.agent-harness.env"} {
		text = removeTOMLSection(text, section)
	}
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n\n") {
		text += "\n"
	}
	text += codexGlobalBlock(req)
	existing, _ := os.ReadFile(path)
	if string(existing) == text {
		return file, nil
	}
	if req.DryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func codexGlobalBlock(req port.NativeInstallRequest) string {
	return fmt.Sprintf(`[mcp_servers.agent_harness]
command = %s
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = %s
`, installutil.TOMLString(req.BinPath), installutil.TOMLString(req.Root))
}

func codexTemplate(req port.NativeInstallRequest) string {
	return `[mcp_servers.agent_harness]
command = "./bin/agent-harness"
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = "."
`
}

func codexHooksConfig(binPath string) map[string]any {
	hooks := map[string]any{}
	for _, spec := range codexLifecycleHookSpecs(binPath) {
		hooks[spec.Event] = []any{codexHookGroup(spec)}
	}
	return map[string]any{"hooks": hooks}
}

type codexLifecycleHookSpec struct {
	BinPath    string
	Event      string
	Subcommand string
	Timeout    int
}

func codexLifecycleHookSpecs(binPath string) []codexLifecycleHookSpec {
	return []codexLifecycleHookSpec{
		{BinPath: binPath, Event: "SessionStart", Subcommand: "session-start", Timeout: 5},
		{BinPath: binPath, Event: "UserPromptSubmit", Subcommand: "user-prompt", Timeout: 5},
		{BinPath: binPath, Event: "PreToolUse", Subcommand: "pre-tool-use", Timeout: 5},
		{BinPath: binPath, Event: "PostToolUse", Subcommand: "post-tool-use", Timeout: 5},
		{BinPath: binPath, Event: "PreCompact", Subcommand: "pre-compact", Timeout: 5},
		{BinPath: binPath, Event: "PostCompact", Subcommand: "post-compact", Timeout: 5},
		{BinPath: binPath, Event: "Stop", Subcommand: "stop", Timeout: 5},
	}
}

func codexHookGroup(spec codexLifecycleHookSpec) map[string]any {
	return map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": codexHookCommand(spec.BinPath, spec.Subcommand),
				"timeout": spec.Timeout,
			},
		},
	}
}

func codexHookCommand(binPath, subcommand string) string {
	cmd := fmt.Sprintf("%s hook %s", shellQuote(binPath), subcommand)
	// Events whose additionalContext is rendered in the Codex TUI pass --host
	// codex so the readable catalog view is used and systemMessage is omitted.
	switch subcommand {
	case "user-prompt", "session-start", "post-compact":
		cmd += " --host codex"
	}
	return cmd
}

func mergeHookConfig(config map[string]any, binPath string) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	for _, spec := range codexLifecycleHookSpecs(binPath) {
		groups := []any{}
		if existing, ok := hooks[spec.Event].([]any); ok {
			for _, group := range existing {
				if !hookGroupContainsAgentHarness(group) {
					groups = append(groups, group)
				}
			}
		}
		groups = append(groups, codexHookGroup(spec))
		hooks[spec.Event] = groups
	}
	return config
}

func hookGroupContainsAgentHarness(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "harness") && strings.Contains(cmd, " hook ") {
			return true
		}
	}
	return false
}

func shellQuote(value string) string {
	if value == "" {
		return "harness"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func removeTOMLSection(src, section string) string {
	marker := "[" + section + "]"
	for {
		pos := strings.Index(src, marker)
		if pos < 0 {
			return src
		}
		next := strings.Index(src[pos+len(marker):], "\n[")
		if next < 0 {
			src = strings.TrimRight(src[:pos], " \t\r\n") + "\n"
			continue
		}
		nextPos := pos + len(marker) + next + 1
		src = src[:pos] + src[nextPos:]
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
