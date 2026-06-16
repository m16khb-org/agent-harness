package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

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
	case "pre-tool-use":
		cmd += " " + installutil.PreToolUseEnforcementFlags()
	case "stop":
		cmd += " " + installutil.StopEnforcementFlags()
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
				if hookGroupHasHooks(group) && !installutil.HookGroupContainsAgentHarness(group) {
					groups = append(groups, group)
				}
			}
		}
		groups = append(groups, codexHookGroup(spec))
		hooks[spec.Event] = groups
	}
	return config
}

func hookGroupHasHooks(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	return ok && len(hooks) > 0
}
