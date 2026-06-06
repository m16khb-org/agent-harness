package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

func writeClaudeSettings(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "claude_user_settings"}
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return file, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	return installutil.WriteJSONPlan(path, file.Kind, mergeClaudeHookConfig(config, req.BinPath), 0o644, req.DryRun)
}

func claudeSettingsConfig(binPath string) map[string]any {
	return mergeClaudeHookConfig(map[string]any{}, binPath)
}

func mergeClaudeHookConfig(config map[string]any, binPath string) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	for _, spec := range claudeLifecycleHookSpecs(binPath) {
		groups := []any{}
		if existing, ok := hooks[spec.Event].([]any); ok {
			for _, group := range existing {
				if !claudeHookGroupContainsAgentHarness(group) {
					groups = append(groups, group)
				}
			}
		}
		groups = append(groups, claudeHookGroup(spec))
		hooks[spec.Event] = groups
	}
	return config
}

type claudeLifecycleHookSpec struct {
	BinPath    string
	Event      string
	Subcommand string
	Matcher    string
	Timeout    int
}

func claudeLifecycleHookSpecs(binPath string) []claudeLifecycleHookSpec {
	return []claudeLifecycleHookSpec{
		{BinPath: binPath, Event: "SessionStart", Subcommand: "session-start", Timeout: 5},
		{BinPath: binPath, Event: "UserPromptSubmit", Subcommand: "user-prompt", Timeout: 5},
		{BinPath: binPath, Event: "PreToolUse", Subcommand: "pre-tool-use", Matcher: "*", Timeout: 5},
		{BinPath: binPath, Event: "PostToolUse", Subcommand: "post-tool-use", Matcher: "*", Timeout: 5},
		{BinPath: binPath, Event: "PreCompact", Subcommand: "pre-compact", Timeout: 5},
		{BinPath: binPath, Event: "PostCompact", Subcommand: "post-compact", Timeout: 5},
		{BinPath: binPath, Event: "Stop", Subcommand: "stop", Timeout: 5},
	}
}

func claudeHookGroup(spec claudeLifecycleHookSpec) map[string]any {
	group := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": claudeHookCommand(spec.BinPath, spec.Subcommand),
				"timeout": spec.Timeout,
			},
		},
	}
	if spec.Matcher != "" {
		group["matcher"] = spec.Matcher
	}
	return group
}

func claudeHookCommand(binPath, subcommand string) string {
	cmd := fmt.Sprintf("%s hook %s", shellQuote(binPath), subcommand)
	if subcommand == "pre-tool-use" {
		cmd += " --host claude --enforce-worktree --enforce-korean-remote-artifacts --enforce-vcs-issue-linking --enforce-staged-checks --enforce-gitops-kubectl"
	}
	if subcommand == "stop" {
		cmd += " --host claude --enforce-numbered-next-actions --relay-next-action-judgement"
	}
	return cmd
}

func claudeHookGroupContainsAgentHarness(group any) bool {
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
