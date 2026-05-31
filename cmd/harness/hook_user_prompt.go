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
	case "session-start":
		return runHookSessionStart(args[1:])
	case "stop":
		return runHookStop(args[1:])
	default:
		hookUsage()
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func hookUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness hook user-prompt [--prompt TEXT] [--host codex|claude] [--json]
  agent-harness hook post-tool-use [--repo PATH] [--json]
  agent-harness hook pre-compact [--repo PATH] [--json]
  agent-harness hook post-compact [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook session-start [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook stop [--repo PATH] [--json]
`)
}

func runHookUserPrompt(args []string) error {
	fs := flag.NewFlagSet("hook user-prompt", flag.ContinueOnError)
	promptFlag := fs.String("prompt", "", "user prompt text; defaults to hook stdin JSON prompt")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
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
	// The stable project-doc catalog now ships via SessionStart/PostCompact, so
	// UserPromptSubmit only carries the small, dynamic per-turn hints. There is no
	// catalog to render, so the output is host-neutral (no systemMessage). The
	// --host flag is still accepted for backward-compatible install commands.
	_ = hostFlag
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
	// Codex PreCompact only accepts the stop-control shape; hookSpecificOutput
	// makes Codex report "invalid PreCompact hook JSON output". The capsule was
	// already persisted above, so host stdout can stay a no-op object.
	return printJSON(map[string]any{})
}

func runHookPostCompact(args []string) error {
	fs := flag.NewFlagSet("hook post-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
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
	// Re-establish the project-doc catalog after compaction, alongside the
	// lifecycle reminder. Compaction drops the SessionStart catalog injection.
	cat := core.BuildProjectDocCatalogContext(parsedRepo)
	if hostOf(hostFlag) == "codex" {
		context := strings.TrimSpace(result.AdditionalContext)
		if cat.ShouldInject {
			context = strings.TrimSpace(cat.UserView)
			if strings.TrimSpace(result.AdditionalContext) != "" {
				context = strings.TrimSpace(result.AdditionalContext) + "\n" + context
			}
		}
		if context == "" {
			return printJSON(map[string]any{})
		}
		// Codex PostCompact accepts only the compact-control schema
		// (continue/stopReason/suppressOutput/systemMessage). Unlike
		// SessionStart, it rejects hookSpecificOutput/additionalContext.
		return printJSON(map[string]any{"systemMessage": context})
	}
	if cat.ShouldInject {
		return emitCatalogPayload("PostCompact", hostOf(hostFlag), cat, result.AdditionalContext)
	}
	return printJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostCompact",
			"additionalContext": result.AdditionalContext,
		},
	})
}

func runHookSessionStart(args []string) error {
	fs := flag.NewFlagSet("hook session-start", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
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
	cat := core.BuildProjectDocCatalogContext(parsedRepo)
	if *jsonOut {
		return printJSON(cat)
	}
	// On compaction Claude Code fires SessionStart with source=compact AND the
	// PostCompact hook; let PostCompact own that case to avoid double injection.
	if !cat.ShouldInject || sourceFromHookInput(stdin) == "compact" {
		return printJSON(map[string]any{
			"hookSpecificOutput": map[string]any{"hookEventName": "SessionStart"},
		})
	}
	return emitCatalogPayload("SessionStart", hostOf(hostFlag), cat, "")
}

// emitCatalogPayload writes the host-aware project-doc catalog injection. The
// model-facing additionalContext stays hidden on Claude Code (paired with a
// pretty systemMessage) while Codex renders additionalContext in its TUI, so the
// readable view is placed there and systemMessage is omitted. prefix, when set
// (PostCompact lifecycle reminder), is prepended to additionalContext.
func emitCatalogPayload(eventName, host string, cat core.ProjectDocCatalogContext, prefix string) error {
	additionalContext := cat.Compact
	if host == "codex" {
		additionalContext = cat.UserView
	}
	if strings.TrimSpace(prefix) != "" {
		additionalContext = prefix + "\n" + additionalContext
	}
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": additionalContext,
		},
	}
	if host != "codex" && cat.UserView != "" {
		payload["systemMessage"] = cat.UserView
	}
	return printJSON(payload)
}

func hostOf(hostFlag *string) string {
	return strings.ToLower(strings.TrimSpace(*hostFlag))
}

func sourceFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	if value, ok := obj["source"].(string); ok {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
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
	// Codex and Claude Stop hooks only accept the stop-control schema
	// (for example decision/reason) or an empty object. Unlike prompt/compact
	// hooks, Stop cannot inject additionalContext; returning hookSpecificOutput
	// makes Codex report "invalid stop hook JSON output". Keep the raw
	// reminder available behind --json, but emit a no-op host payload here.
	return printJSON(map[string]any{})
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
