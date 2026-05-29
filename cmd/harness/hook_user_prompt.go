package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"agent-harness/internal/core"
)

func runHook(args []string) error {
	if len(args) == 0 {
		hookUsage()
		return fmt.Errorf("missing hook subcommand")
	}
	switch args[0] {
	case "user-prompt":
		return runHookUserPrompt(args[1:])
	case "post-tool-use":
		return runHookPostToolUse(args[1:])
	case "pre-compact":
		return runHookPreCompact(args[1:])
	case "post-compact":
		return runHookPostCompact(args[1:])
	case "stop":
		return runHookStop(args[1:])
	default:
		hookUsage()
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func hookUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness hook user-prompt [--prompt TEXT] [--json]
  agent-harness hook post-tool-use [--repo PATH] [--json]
  agent-harness hook pre-compact [--repo PATH] [--json]
  agent-harness hook post-compact [--repo PATH] [--json]
  agent-harness hook stop [--repo PATH] [--json]
`)
}

func runHookUserPrompt(args []string) error {
	fs := flag.NewFlagSet("hook user-prompt", flag.ContinueOnError)
	promptFlag := fs.String("prompt", "", "user prompt text; defaults to hook stdin JSON prompt")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	prompt := strings.TrimSpace(*promptFlag)
	if prompt == "" && fs.NArg() > 0 {
		prompt = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	stdin, _ := io.ReadAll(os.Stdin)
	if prompt == "" {
		prompt = promptFromHookInput(stdin)
	}
	repo := repoFromHookInput(stdin)
	if repo == "" {
		repo = resolveTarget("")
	}
	result := core.BuildUserPromptMCPHints(core.HookUserPromptRequest{Prompt: prompt, Repo: repo})
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": result.AdditionalContext,
		},
	})
}

func promptFromHookInput(input []byte) string {
	if len(strings.TrimSpace(string(input))) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return strings.TrimSpace(string(input))
	}
	for _, key := range []string{"prompt", "user_prompt", "message", "text"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if nested, ok := obj["hook_input"].(map[string]any); ok {
		if value, ok := nested["prompt"].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func repoFromHookInput(input []byte) string {
	if len(strings.TrimSpace(string(input))) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"repo", "cwd", "workspace", "workspace_root", "project_dir"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if nested, ok := obj["hook_input"].(map[string]any); ok {
		for _, key := range []string{"repo", "cwd", "workspace", "workspace_root", "project_dir"} {
			if value, ok := nested[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func runHookPostToolUse(args []string) error {
	fs := flag.NewFlagSet("hook post-tool-use", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = repoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = resolveTarget("")
	}
	result, err := core.RecordLifecycleToolUse(core.HookToolUseLifecycleRequest{
		Repo:    parsedRepo,
		Tool:    toolNameFromHookInput(stdin),
		Paths:   pathsFromHookInput(stdin),
		Command: commandFromHookInput(stdin),
		Source:  "post-tool-use",
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PostToolUse",
		},
	})
}

func runHookPreCompact(args []string) error {
	fs := flag.NewFlagSet("hook pre-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = repoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = resolveTarget("")
	}
	result := core.BuildLifecyclePreCompactCapsule(parsedRepo)
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PreCompact",
		},
	})
}

func runHookPostCompact(args []string) error {
	fs := flag.NewFlagSet("hook post-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = repoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = resolveTarget("")
	}
	result := core.BuildLifecyclePostCompactReminder(parsedRepo)
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostCompact",
			"additionalContext": result.AdditionalContext,
		},
	})
}

func runHookStop(args []string) error {
	fs := flag.NewFlagSet("hook stop", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = repoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = resolveTarget("")
	}
	result := core.BuildLifecycleStopReminder(parsedRepo)
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "Stop",
			"additionalContext": result.AdditionalContext,
		},
	})
}

func toolNameFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"tool_name", "tool", "name"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func commandFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"command", "cmd"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if toolInput, ok := obj["tool_input"].(map[string]any); ok {
		for _, key := range []string{"command", "cmd"} {
			if value, ok := toolInput[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func pathsFromHookInput(input []byte) []string {
	obj := hookInputObject(input)
	seen := map[string]bool{}
	out := []string{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				lk := strings.ToLower(k)
				if lk == "path" || strings.HasSuffix(lk, "_path") || lk == "file" || lk == "filename" {
					if s, ok := v.(string); ok {
						addHookPath(&out, seen, s)
					}
				}
				walk(v)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		case string:
			if strings.Contains(x, ".go") || strings.Contains(x, ".agent-harness") || strings.Contains(x, "testdata/") {
				addHookPath(&out, seen, x)
			}
		}
	}
	walk(obj)
	return out
}

func addHookPath(out *[]string, seen map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value == "" || seen[value] {
		return
	}
	seen[value] = true
	*out = append(*out, value)
}

func hookInputObject(input []byte) map[string]any {
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return map[string]any{}
	}
	if nested, ok := obj["hook_input"].(map[string]any); ok {
		for k, v := range nested {
			if _, exists := obj[k]; !exists {
				obj[k] = v
			}
		}
	}
	return obj
}
