package claude

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

	mcpConfig := claudeProjectMCPConfig(req)
	file, err := installutil.WriteJSONPlan(filepath.Join(req.Root, "configs", "claude", "mcp.project.json"), "claude_project_mcp_template", mcpConfig, 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	projectHookSettings := claudeProjectHookSettings()
	file, err = installutil.WriteJSONPlan(filepath.Join(req.Root, "configs", "claude", "hooks", "session-start-llm-wiki.settings.json"), "claude_project_session_start_hook_template", projectHookSettings, 0o644, req.DryRun)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	if req.ClaudeUserHook {
		userHookSettings := claudeUserHookSettings(req)
		file, err = mergeClaudeSettings(filepath.Join(req.Home, ".claude", "settings.json"), userHookSettings, "claude_user_settings", req.DryRun)
		result.Files = append(result.Files, file)
		if err != nil {
			errs = append(errs, err)
		}
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
		file, err = mergeClaudeSettings(filepath.Join(req.Root, ".claude", "settings.json"), projectHookSettings, "claude_project_settings", req.DryRun)
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

func claudeProjectMCPConfig(req port.NativeInstallRequest) map[string]any {
	portableRoot := req.PortableLLMWikiRoot
	if portableRoot == "" {
		portableRoot = "~/workspace/knowledge-base/llm-wiki"
	}
	return map[string]any{
		"mcpServers": map[string]any{
			"agent-harness": map[string]any{
				"type":    "stdio",
				"command": "./bin/harness",
				"args":    []string{"mcp"},
				"env": map[string]any{
					"HARNESS_ROOT":  ".",
					"LLM_WIKI_ROOT": portableRoot,
				},
			},
		},
	}
}

func claudeProjectHookSettings() map[string]any {
	entry := claudeHookEntry("./scripts/session-start-llm-wiki.sh")
	return map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{entry},
		},
	}
}

func claudeUserHookSettings(req port.NativeInstallRequest) map[string]any {
	command := "HARNESS_ROOT=" + shellQuote(req.Root) + " " + shellQuote(filepath.Join(req.Root, "scripts", "session-start-llm-wiki.sh"))
	return map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{claudeHookEntry(command)},
		},
	}
}

func claudeHookEntry(command string) map[string]any {
	return map[string]any{
		"matcher": "startup|resume|clear|compact",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	}
}

func mergeClaudeSettings(path string, hookSettings map[string]any, kind string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	settings := map[string]any{}
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &settings); err != nil {
			return file, fmt.Errorf("refusing to merge invalid JSON in %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) && !dryRun {
		return file, err
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	entry := hookSettingsEntry(hookSettings)
	sessionStart := normalizeJSONArray(hooks["SessionStart"])
	if !containsJSON(sessionStart, entry) {
		sessionStart = append(sessionStart, entry)
	}
	hooks["SessionStart"] = sessionStart
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return file, err
	}
	b = append(b, '\n')
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(b) {
		return file, nil
	}
	if dryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func hookSettingsEntry(hookSettings map[string]any) map[string]any {
	hooks, _ := hookSettings["hooks"].(map[string]any)
	items := normalizeJSONArray(hooks["SessionStart"])
	if len(items) == 0 {
		return claudeHookEntry("./scripts/session-start-llm-wiki.sh")
	}
	if entry, ok := items[0].(map[string]any); ok {
		return entry
	}
	return claudeHookEntry("./scripts/session-start-llm-wiki.sh")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func normalizeJSONArray(value any) []any {
	switch v := value.(type) {
	case []any:
		return append([]any{}, v...)
	case nil:
		return []any{}
	default:
		return []any{v}
	}
}

func containsJSON(items []any, want any) bool {
	wantBytes, _ := json.Marshal(want)
	for _, item := range items {
		itemBytes, _ := json.Marshal(item)
		if string(itemBytes) == string(wantBytes) {
			return true
		}
	}
	return false
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
