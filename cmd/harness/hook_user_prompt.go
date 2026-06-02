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
	stdin, restoreStdin, stdinErr := captureReplayableHookStdin()
	if restoreStdin != nil {
		defer restoreStdin()
	}
	if stdinErr != nil {
		return stdinErr
	}
	err := runHookDispatch(args)
	if err != nil {
		recordHookFailure(args, stdin, err)
	}
	return err
}

func captureReplayableHookStdin() ([]byte, func(), error) {
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
		return nil, nil, nil
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, nil, fmt.Errorf("read hook stdin: %w", err)
	}
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write(stdin)
		_ = w.Close()
	}()
	return stdin, func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}, nil
}

func runHookDispatch(args []string) error {
	if len(args) == 0 {
		hookUsage()
		return fmt.Errorf("missing hook subcommand")
	}
	switch args[0] {
	case "user-prompt":
		return runHookUserPrompt(args[1:])
	case "pre-tool-use":
		return runHookPreToolUse(args[1:])
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
	case "failures":
		return runHookFailures(args[1:])
	default:
		hookUsage()
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func recordHookFailure(args []string, stdin []byte, hookErr error) {
	hook := "unknown"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		hook = strings.TrimSpace(args[0])
	}
	cwd, _ := os.Getwd()
	repo := hookArgValue(args, "--repo")
	if repo == "" {
		repo = repoFromHookInput(stdin)
	}
	_, _ = core.RecordHookFailureEvent(core.HookFailureEvent{
		Hook:           hook,
		Host:           hookArgValue(args, "--host"),
		Repo:           repo,
		CWD:            cwd,
		Tool:           toolNameFromHookInput(stdin),
		Argv:           args,
		CommandSnippet: commandFromHookInput(stdin),
		Error:          hookErr.Error(),
	})
}

func hookArgValue(args []string, flagName string) string {
	prefix := flagName + "="
	for i, arg := range args {
		if arg == flagName && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}

func hookUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness hook user-prompt [--prompt TEXT] [--host codex|claude] [--enable-agy-hints] [--json]
  agent-harness hook pre-tool-use [--repo PATH] [--host codex|claude] [--enforce-search-routing] [--json]
  agent-harness hook post-tool-use [--repo PATH] [--json]
  agent-harness hook pre-compact [--repo PATH] [--json]
  agent-harness hook post-compact [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook session-start [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook stop [--repo PATH] [--json]
  agent-harness hook failures [--limit N] [--json]
`)
}

func runHookFailures(args []string) error {
	fs := flag.NewFlagSet("hook failures", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "maximum recent hook failure events to return")
	jsonOut := fs.Bool("json", false, "print hook failure events as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.ListHookFailureEvents(*limit)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(result)
}

func runHookUserPrompt(args []string) error {
	fs := flag.NewFlagSet("hook user-prompt", flag.ContinueOnError)
	promptFlag := fs.String("prompt", "", "user prompt text; defaults to hook stdin JSON prompt")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	enableAgyHints := fs.Bool("enable-agy-hints", false, "suggest agy -p for LLM second-pass review when the prompt fits")
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
	result := core.BuildUserPromptMCPHints(core.HookUserPromptRequest{Prompt: prompt, Repo: repo, EnableAgyHints: *enableAgyHints || envBool("HARNESS_ENABLE_AGY_HINTS")})
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

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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

func runHookPreToolUse(args []string) error {
	fs := flag.NewFlagSet("hook pre-tool-use", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	host := fs.String("host", "", "hook host (codex or claude); controls host-specific block schema")
	enforceSearchRouting := fs.Bool("enforce-search-routing", false, "block obvious CodeGraph/rg search routing mismatches")
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
	result := core.BuildLifecyclePreToolUseDecision(core.HookToolUseLifecycleRequest{
		Repo:                 parsedRepo,
		Tool:                 toolNameFromHookInput(stdin),
		Paths:                pathsFromHookInput(stdin),
		Command:              commandFromHookInput(stdin),
		Source:               "pre-tool-use",
		EnforceSearchRouting: *enforceSearchRouting,
	})
	if *jsonOut {
		return printJSON(result)
	}
	if result.Decision == "block" {
		if strings.EqualFold(strings.TrimSpace(*host), "claude") {
			return printJSON(map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName":            "PreToolUse",
					"permissionDecision":       "deny",
					"permissionDecisionReason": result.Reason,
				},
			})
		}
		return printJSON(map[string]any{
			"decision": result.Decision,
			"reason":   result.Reason,
		})
	}
	// PreToolUse is on the critical path before every tool call. Keep the shared
	// harness hook cheap and non-blocking by default.
	return printJSON(map[string]any{})
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
	tool := toolNameFromHookInput(stdin)
	paths := pathsFromHookInput(stdin)
	command := commandFromHookInput(stdin)
	result, err := core.RecordLifecycleToolUse(core.HookToolUseLifecycleRequest{
		Repo:    parsedRepo,
		Tool:    tool,
		Paths:   paths,
		Command: command,
		Source:  "post-tool-use",
	})
	if err != nil {
		result = core.HookToolUseLifecycleResult{OK: false, Warnings: []string{"lifecycle_record_error:" + err.Error()}}
	}
	var draftQueue *core.DraftWikiQueueAppendResult
	draftMaterial := draftWikiMaterialFromHookInput(stdin)
	if draftMaterial != "" {
		queued, queueErr := core.AppendDraftWikiQueueEvent(core.DraftWikiQueueAppendRequest{
			RepoRoot:       parsedRepo,
			Tool:           tool,
			Command:        command,
			Paths:          paths,
			SourceMaterial: draftMaterial,
			Source:         "post-tool-use",
		})
		if queueErr == nil {
			draftQueue = &queued
		} else {
			result.Warnings = append(result.Warnings, "draft_wiki_queue_error:"+queueErr.Error())
		}
	}
	if *jsonOut {
		out := map[string]any{
			"ok":        result.OK,
			"lifecycle": result,
		}
		if draftQueue != nil {
			out["draft_wiki_queue"] = draftQueue
		}
		return printJSON(out)
	}
	// Codex PostToolUse is on the critical path after every tool call and does
	// not need to inject context. Keep host stdout in the broad no-op schema so
	// lifecycle bookkeeping can never surface as a hook failure in the UI.
	return printJSON(map[string]any{})
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
		if context == "" && cat.ShouldInject {
			context = strings.TrimSpace(cat.UserView)
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
		for _, key := range []string{"query", "pattern", "symbol", "text", "q"} {
			if value, ok := toolInput[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func draftWikiMaterialFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	var parts []string
	var walk func(any, string)
	walk = func(v any, key string) {
		switch x := v.(type) {
		case map[string]any:
			for k, value := range x {
				walk(value, strings.ToLower(k))
			}
		case []any:
			for _, item := range x {
				walk(item, key)
			}
		case string:
			if draftWikiHookMaterialKey(key) && strings.TrimSpace(x) != "" {
				parts = append(parts, strings.TrimSpace(x))
			}
		}
	}
	walk(obj, "")
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func draftWikiHookMaterialKey(key string) bool {
	switch key {
	case "tool_response", "tool_result", "result", "response", "output", "content", "text", "observation", "observations":
		return true
	default:
		return false
	}
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
