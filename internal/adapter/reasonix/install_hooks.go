package reasonix

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

func writeReasonixSettings(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "reasonix_user_settings"}
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return file, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	return installutil.WriteJSONPlan(path, file.Kind, mergeReasonixHookConfig(config, req.BinPath), 0o644, req.DryRun)
}

func reasonixSettingsConfig(binPath string) map[string]any {
	return mergeReasonixHookConfig(map[string]any{}, binPath)
}

func mergeReasonixHookConfig(config map[string]any, binPath string) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	for _, spec := range reasonixLifecycleHookSpecs(binPath) {
		groups := []any{}
		if existing, ok := hooks[spec.Event].([]any); ok {
			for _, group := range existing {
				if !reasonixHookGroupContainsAgentHarness(group) {
					groups = append(groups, group)
				}
			}
		}
		groups = append(groups, reasonixHookGroup(spec))
		hooks[spec.Event] = groups
	}
	return config
}

type reasonixLifecycleHookSpec struct {
	BinPath    string
	Event      string
	Subcommand string
	Matcher    string
	Timeout    int
}

func reasonixLifecycleHookSpecs(binPath string) []reasonixLifecycleHookSpec {
	return []reasonixLifecycleHookSpec{
		{BinPath: binPath, Event: "SessionStart", Subcommand: "session-start", Timeout: 5},
		{BinPath: binPath, Event: "SessionEnd", Subcommand: "session-end", Timeout: 5},
		{BinPath: binPath, Event: "PromptSubmit", Subcommand: "user-prompt", Timeout: 5},
		{BinPath: binPath, Event: "PreToolUse", Subcommand: "pre-tool-use", Matcher: "*", Timeout: 5},
		{BinPath: binPath, Event: "PostToolUse", Subcommand: "post-tool-use", Matcher: "*", Timeout: 5},
		{BinPath: binPath, Event: "PreCompact", Subcommand: "pre-compact", Timeout: 5},
		{BinPath: binPath, Event: "Stop", Subcommand: "stop", Timeout: 5},
	}
}

func reasonixHookGroup(spec reasonixLifecycleHookSpec) map[string]any {
	group := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": reasonixHookCommand(spec.BinPath, spec.Subcommand),
				"timeout": spec.Timeout,
			},
		},
	}
	if spec.Matcher != "" {
		group["matcher"] = spec.Matcher
	}
	return group
}

func reasonixHookCommand(binPath, subcommand string) string {
	cmd := fmt.Sprintf("%s hook %s", shellQuote(binPath), subcommand)
	switch subcommand {
	case "pre-tool-use":
		cmd += " --host reasonix --enforce-worktree --enforce-korean-remote-artifacts --enforce-vcs-issue-linking --enforce-staged-checks --enforce-gitops-kubectl"
	case "stop":
		cmd += " --host reasonix --enforce-numbered-next-actions --relay-next-action-judgement"
	case "session-start":
		cmd += " --host reasonix"
	case "session-end":
		cmd += " --host reasonix"
	case "user-prompt":
		cmd += " --host reasonix"
	case "post-tool-use":
		// --host lets post-tool-use inject a deterministic gofmt lint-failure as
		// additionalContext (B3); Codex omits --host so it keeps its no-op shape.
		cmd += " --host reasonix"
	}
	return cmd
}

func reasonixHookGroupContainsAgentHarness(group any) bool {
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
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
