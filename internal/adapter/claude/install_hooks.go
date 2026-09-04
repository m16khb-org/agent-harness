package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"issueops/internal/port"
)

func writeClaudeSettings(path string, req port.NativeInstallRequest) (port.InstallFile, []string, error) {
	file := port.InstallFile{Path: path, Kind: "claude_user_settings"}
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return file, nil, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, nil, err
	}
	if err := ValidateHookConfigForMerge(config, claudeLifecycleHookEvents); err != nil {
		return file, nil, err
	}
	messages := HookTargetDriftMessages(config, "claude", req.BinPath)
	// 경로가 같아도 빌드 세대가 갈리면 이전 세대 hook이 새 typed command를
	// 모른 채 차단해 복구가 교착된다(#328). 그 축을 여기서 함께 보고한다.
	if HookTargetGenerationMessages != nil && RunningBuildGenerationString != nil && FileBuildGenerationString != nil {
		messages = append(messages, HookTargetGenerationMessages(config, "claude", req.BinPath, RunningBuildGenerationString(), FileBuildGenerationString)...)
	}
	written, err := WriteJSONPlan(path, file.Kind, mergeClaudeHookConfig(config, req.BinPath), 0o644, req.DryRun)
	return written, messages, err
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
	desired := map[string]claudeLifecycleHookSpec{}
	for _, spec := range claudeLifecycleHookSpecs(binPath) {
		desired[spec.Event] = spec
	}
	for _, event := range claudeLifecycleHookEvents {
		groups := []any{}
		if existing, ok := hooks[event].([]any); ok {
			for _, group := range existing {
				if !HookGroupContainsAgentHarness(group) && !HookGroupContainsCommand(group, shellQuote(binPath)+" hook ") {
					groups = append(groups, group)
				}
			}
		}
		if spec, ok := desired[event]; ok {
			groups = append(groups, claudeHookGroup(spec))
		}
		if len(groups) == 0 {
			delete(hooks, event)
			continue
		}
		hooks[event] = groups
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
	// SessionStart alone carries the catalog: Claude re-runs it with
	// source "compact" after compaction, while PostCompact output is only a
	// user display string there (verified against Claude Code 2.1.247).
	return []claudeLifecycleHookSpec{
		{BinPath: binPath, Event: "SessionStart", Subcommand: "session-start", Timeout: 5},
	}
}

var claudeLifecycleHookEvents = []string{
	"SessionStart",
	"UserPromptSubmit",
	"PreToolUse",
	"PostToolUse",
	"PreCompact",
	"PostCompact",
	"Stop",
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
	if subcommand == "session-start" || subcommand == "post-compact" {
		cmd += " --host claude"
	}
	return cmd
}
