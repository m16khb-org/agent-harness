package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/port"
)

func writeCodexHooks(path string, req port.NativeInstallRequest) (port.InstallFile, []string, error) {
	file := port.InstallFile{Path: path, Kind: "codex_user_hooks_config"}
	config := map[string]any{}
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &config); err != nil {
			return file, nil, err
		}
	} else if err != nil && !os.IsNotExist(err) && !req.DryRun {
		return file, nil, err
	}
	if err := ValidateHookConfigForMerge(config, codexLifecycleHookEvents); err != nil {
		return file, nil, err
	}
	messages := HookTargetDriftMessages(config, "codex", req.BinPath)
	// 경로가 같아도 빌드 세대가 갈리면 이전 세대 hook이 새 typed command를
	// 모른 채 차단해 복구가 교착된다(#328). 그 축을 여기서 함께 보고한다.
	if HookTargetGenerationMessages != nil && RunningBuildGenerationString != nil && FileBuildGenerationString != nil {
		messages = append(messages, HookTargetGenerationMessages(config, "codex", req.BinPath, RunningBuildGenerationString(), FileBuildGenerationString)...)
	}
	merged := mergeHookConfig(config, req.BinPath)
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return file, messages, err
	}
	text := string(append(b, '\n'))
	if existing, err := os.ReadFile(path); err == nil && string(existing) == text {
		return file, messages, nil
	}
	if req.DryRun {
		file.WouldWrite = true
		return file, messages, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, messages, err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return file, messages, err
	}
	file.Written = true
	return file, messages, nil
}

func codexHooksConfig(binPath string) map[string]any {
	hooks := map[string]any{}
	for _, spec := range codexLifecycleHookSpecs(binPath) {
		hooks[spec.Event] = []any{codexHookGroup(spec)}
	}
	return map[string]any{"hooks": hooks}
}

func HooksConfig(binPath string) map[string]any {
	return codexHooksConfig(binPath)
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
		{BinPath: binPath, Event: "PostCompact", Subcommand: "post-compact", Timeout: 5},
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
	// additionalContext가 Codex TUI에 렌더링되는 이벤트는 --host codex를 넘겨
	// 읽기 쉬운 catalog 뷰를 쓰고 systemMessage는 생략한다.
	switch subcommand {
	case "session-start", "post-compact":
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
	desired := map[string]codexLifecycleHookSpec{}
	for _, spec := range codexLifecycleHookSpecs(binPath) {
		desired[spec.Event] = spec
	}
	for _, event := range codexLifecycleHookEvents {
		groups := []any{}
		replaced := false
		if existing, ok := hooks[event].([]any); ok {
			for _, group := range existing {
				if !hookGroupHasHooks(group) {
					continue
				}
				if HookGroupContainsAgentHarness(group) || HookGroupContainsCommand(group, shellQuote(binPath)+" hook ") {
					if spec, desiredEvent := desired[event]; desiredEvent && !replaced {
						groups = append(groups, codexHookGroup(spec))
						replaced = true
					}
					continue
				}
				groups = append(groups, group)
			}
		}
		if spec, ok := desired[event]; ok && !replaced {
			groups = append(groups, codexHookGroup(spec))
		}
		if len(groups) == 0 {
			delete(hooks, event)
			continue
		}
		hooks[event] = groups
	}
	return config
}

var codexLifecycleHookEvents = []string{
	"SessionStart",
	"UserPromptSubmit",
	"PreToolUse",
	"PostToolUse",
	"PreCompact",
	"PostCompact",
	"Stop",
}

func hookGroupHasHooks(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	return ok && len(hooks) > 0
}
