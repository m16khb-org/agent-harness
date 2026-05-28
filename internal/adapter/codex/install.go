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

	for _, skillName := range req.SkillNames {
		link, err := installutil.EnsureSymlinkPlan(filepath.Join(req.Root, "skills", skillName), filepath.Join(req.CodexHome, "skills", skillName), req.DryRun)
		result.Links = append(result.Links, link)
		if err != nil {
			errs = append(errs, err)
		}
	}

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
	file, err = installutil.WriteJSONPlan(hooksTemplatePath, "codex_hooks_template", codexHooksConfig("./bin/harness"), 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	if req.DryRun {
		result.Messages = append(result.Messages, "dry-run: planned Codex user skill links, MCP config, and UserPromptSubmit hook without writing")
	}

	if len(errs) > 0 {
		result.OK = false
		return result, joinErrors(errs)
	}
	return result, nil
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
	merged := mergeHookConfig(config, codexHookCommand(req.BinPath))
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
command = "./bin/harness"
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = "."
`
}

func codexHooksConfig(binPath string) map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": codexHookCommand(binPath),
							"timeout": 5,
						},
					},
				},
			},
		},
	}
}

func codexHookCommand(binPath string) string {
	return fmt.Sprintf("%s hook user-prompt", shellQuote(binPath))
}

func mergeHookConfig(config map[string]any, command string) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	groups := []any{}
	if existing, ok := hooks["UserPromptSubmit"].([]any); ok {
		for _, group := range existing {
			if !hookGroupContainsAgentHarness(group) {
				groups = append(groups, group)
			}
		}
	}
	groups = append(groups, map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
				"timeout": 5,
			},
		},
	})
	hooks["UserPromptSubmit"] = groups
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
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "hook user-prompt") && strings.Contains(cmd, "harness") {
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
