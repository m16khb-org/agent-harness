package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/port"
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
	for _, spec := range claudeLifecycleHookSpecs(binPath) {
		groups := []any{}
		if existing, ok := hooks[spec.Event].([]any); ok {
			for _, group := range existing {
				if !HookGroupContainsAgentHarness(group) {
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
		cmd += " --host claude " + PreToolUseEnforcementFlags()
	}
	if subcommand == "stop" {
		cmd += " --host claude " + StopEnforcementFlags()
	}
	if subcommand == "post-tool-use" {
		// --host lets post-tool-use inject a deterministic gofmt lint-failure as
		// additionalContext (B3); Codex omits --host so it keeps its no-op shape.
		cmd += " --host claude"
	}
	return cmd
}
