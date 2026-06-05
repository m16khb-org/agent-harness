package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
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
  agent-harness hook pre-tool-use [--repo PATH] [--host codex|claude] [--enforce-search-routing] [--enforce-worktree] [--enforce-staged-checks] [--enforce-gitops-kubectl] [--json]
  agent-harness hook post-tool-use [--repo PATH] [--json]
  agent-harness hook pre-compact [--repo PATH] [--json]
  agent-harness hook post-compact [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook session-start [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook stop [--repo PATH] [--host codex|claude] [--enforce-numbered-next-actions] [--relay-next-action-judgement] [--json]
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
	if !isStopHookContinuationPrompt(prompt) {
		core.ClearStopNextActionRelay(repo)
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

func isStopHookContinuationPrompt(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	return strings.HasPrefix(trimmed, `<hook_prompt `) &&
		strings.Contains(trimmed, `hook_run_id="stop:`) &&
		strings.Contains(trimmed, `</hook_prompt>`)
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envFloat(name string) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return value
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
	enforceWorktree := fs.Bool("enforce-worktree", false, "block mutating tool targets outside HARNESS_EXPECTED_WORKTREE or --expected-worktree")
	enforceKoreanRemote := fs.Bool("enforce-korean-remote-artifacts", false, "block gh issue/pr create/edit when title/body fail the IssueOps Korean remote artifact gate")
	enforceVCSLinking := fs.Bool("enforce-vcs-issue-linking", false, "block gh/glab remote create without labels and issue create/edit bodies that violate provider-specific IssueOps linking rules")
	enforceGitOpsKubectl := fs.Bool("enforce-gitops-kubectl", false, "block direct mutating kubectl commands so cluster changes go through GitOps")
	enforceStagedChecks := fs.Bool("enforce-staged-checks", false, "ask before broad lint/format checks that should use staged or changed-file scope")
	expectedWorktree := fs.String("expected-worktree", os.Getenv("HARNESS_EXPECTED_WORKTREE"), "expected isolated IssueOps worktree path")
	sourceCheckout := fs.String("source-checkout", os.Getenv("HARNESS_SOURCE_CHECKOUT"), "source checkout path for diagnostics")
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
		ProjectPath:          projectPathFromHookInput(stdin),
		Source:               "pre-tool-use",
		EnforceSearchRouting: *enforceSearchRouting,
		EnforceWorktree:      *enforceWorktree,
		EnforceKoreanRemote:  *enforceKoreanRemote,
		EnforceVCSLinking:    *enforceVCSLinking,
		EnforceGitOpsKubectl: *enforceGitOpsKubectl,
		EnforceStagedChecks:  *enforceStagedChecks,
		ExpectedWorktree:     *expectedWorktree,
		SourceCheckout:       *sourceCheckout,
	})
	if *jsonOut {
		return printJSON(result)
	}
	if result.Decision == "block" || result.Decision == "ask" {
		hostName := strings.TrimSpace(*host)
		if result.Decision == "ask" || strings.EqualFold(hostName, "claude") {
			permissionDecision := result.Decision
			if permissionDecision == "block" {
				permissionDecision = "deny"
			}
			return printJSON(map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName":            "PreToolUse",
					"permissionDecision":       permissionDecision,
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
	core.ClearStopNextActionRelay(parsedRepo)
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
	if *jsonOut {
		return printJSON(map[string]any{
			"ok":        result.OK,
			"lifecycle": result,
		})
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
	host := fs.String("host", "", "hook host (codex or claude); reserved for host-compatible stop output")
	enforceNumberedNextActions := fs.Bool("enforce-numbered-next-actions", false, "block Stop when the final response lacks 1/2/3 next-action choices")
	relayNextActionJudgement := fs.Bool("relay-next-action-judgement", false, "re-enter the main agent when the final response contains inspectable next-action facts")
	autoProceedNextActions := fs.Bool("auto-proceed-next-actions", false, "deprecated alias for --relay-next-action-judgement")
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
	message := lastAssistantMessageFromHookInput(stdin)
	if message == "" {
		message = readLastAssistantMessageFromTranscript(transcriptPathFromHookInput(stdin))
	}
	nextActions := core.BuildNumberedNextActionsDecision(
		message,
		*enforceNumberedNextActions,
		"stop",
	)
	// --auto-proceed-next-actions is retained only as a compatibility alias. This
	// hook no longer auto-proceeds or judges choices; it detects that an explicit
	// next-action review point exists and relays observed facts back to the main agent.
	stopHookActive := hookInputBool(stdin, "stop_hook_active")
	nextActionTriggerEnabled := *relayNextActionJudgement || *autoProceedNextActions
	nextActionTrigger := core.BuildNextActionJudgementTrigger(message)
	if *jsonOut {
		return printJSON(map[string]any{
			"lifecycle":                    result,
			"numbered_next_actions":        nextActions,
			"next_action_judgement":        nextActionTrigger,
			"next_action_judgement_active": nextActionTriggerEnabled,
		})
	}
	_ = host
	// The external-LLM gate (core.EvaluateNextActionAutoProceedLLM) is intentionally
	// not called here: a synchronous agy/Gemini call measured ~13-25s, which is
	// unusable inside a Stop hook's latency budget. The hook also does not replace
	// that LLM with a local scorer. It reports only that the response reached an
	// explicit next-action judgement point and sends the observed facts to the main
	// agent, which owns safety, reversibility, alignment, and proceed/ask judgement.
	if nextActionTriggerEnabled && nextActionTrigger.ShouldReenterAgent && !stopHookActive {
		relayRecord := core.RecordStopNextActionRelay(parsedRepo, nextActionTrigger)
		if !relayRecord.ShouldRelay {
			return printJSON(map[string]any{})
		}
		return printJSON(map[string]any{
			"continue": true,
			"decision": "block",
			"reason":   nextActionJudgementReason(nextActionTrigger),
		})
	}
	// Block a Stop that lacks numbered next actions, but drive an IN-TURN
	// continuation rather than a hard stop. Verified against the host schemas:
	// Claude 2.1.162 embedded hook docs ("continue - Set to false to block/stop")
	// and Codex 0.137.0 stop.command.output schema both treat continue:false as a
	// hard stop that takes precedence over decision, while decision:"block" + reason
	// makes the agent continue and act on the reason. Sending continue:false here
	// (the prior behavior) caused the agent to halt and surface the block reason to
	// the user instead of presenting the choices itself. Use continue:true like the
	// auto-proceed branch above so the agent stays in-turn and emits the choices.
	//
	// Guard with stop_hook_active: hosts set it true when this Stop is itself a
	// continuation of a prior stop-hook block. The documented anti-loop contract is
	// to allow the stop while it is true, so a non-complying agent cannot loop.
	if nextActions.Decision == "block" && !stopHookActive {
		return printJSON(map[string]any{
			"continue": true,
			"decision": "block",
			"reason":   nextActions.Reason,
		})
	}
	// Codex and Claude Stop hooks only accept the stop-control schema
	// (for example decision/reason/systemMessage) or an empty object. Unlike
	// prompt/compact hooks, Stop cannot inject additionalContext; returning
	// hookSpecificOutput makes Codex report "invalid stop hook JSON output". Keep the
	// raw reminder available behind --json, but emit a no-op host payload here.
	return printJSON(map[string]any{})
}

func nextActionJudgementReason(trigger core.NextActionJudgementTriggerResult) string {
	recommended := "없음"
	if trigger.RecommendedCount == 1 {
		recommended = fmt.Sprintf("%d번 %q", trigger.RecommendedIndex, trigger.RecommendedText)
	} else if trigger.RecommendedCount > 1 {
		recommended = fmt.Sprintf("%d개", trigger.RecommendedCount)
	}
	return fmt.Sprintf("다음 행동 판단 지점에 도달했습니다. 훅이 관찰한 근거: 명시적 선택지 %d개, 추천 선택지 %s. 훅은 안전성, 가역성, 사용자 의도 정합성, 진행 여부를 판단하지 않습니다. 메인 에이전트가 현재 대화와 작업 맥락을 근거로 직접 판단하세요. 사용자 추가 입력이 필요 없고 진행이 맞으면 지금 실행하세요. 사용자 결정이 필요한 경우에만 직전 선택지 중 하나를 골라 달라고 요청한 뒤 멈추세요.", trigger.ChoiceCount, recommended)
}

func lastAssistantMessageFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"last_assistant_message", "lastAssistantMessage", "assistant_message", "assistantMessage", "response", "final_response"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func transcriptPathFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"transcript_path", "transcriptPath"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readLastAssistantMessageFromTranscript(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.Contains(strings.ToLower(line), "assistant") {
			continue
		}
		var obj any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if text := assistantTextFromTranscriptObject(obj); text != "" {
			return text
		}
	}
	return ""
}

func assistantTextFromTranscriptObject(value any) string {
	switch v := value.(type) {
	case map[string]any:
		role := ""
		if r, ok := v["role"].(string); ok {
			role = strings.ToLower(strings.TrimSpace(r))
		}
		if msg, ok := v["message"].(map[string]any); ok {
			if r, ok := msg["role"].(string); ok && role == "" {
				role = strings.ToLower(strings.TrimSpace(r))
			}
		}
		if typ, ok := v["type"].(string); ok && role == "" {
			role = strings.ToLower(strings.TrimSpace(typ))
		}
		if role != "" && role != "assistant" {
			return ""
		}
		for _, key := range []string{"last_assistant_message", "text", "content", "message"} {
			if text := transcriptTextValue(v[key]); text != "" {
				return text
			}
		}
	case []any:
		return transcriptTextValue(v)
	}
	return ""
}

func transcriptTextValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		var parts []string
		for _, item := range v {
			if text := transcriptTextValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		if typ, ok := v["type"].(string); ok && strings.EqualFold(strings.TrimSpace(typ), "tool_use") {
			return ""
		}
		for _, key := range []string{"text", "content"} {
			if text := transcriptTextValue(v[key]); text != "" {
				return text
			}
		}
	}
	return ""
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
		if command := mcpRemoteArtifactCommandFromHookObject(obj, toolInput); command != "" {
			return command
		}
		for _, key := range []string{"query", "pattern", "symbol", "text", "q"} {
			if value, ok := toolInput[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func projectPathFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	if toolInput, ok := obj["tool_input"].(map[string]any); ok {
		for _, key := range []string{"projectPath", "project_path"} {
			if value, ok := toolInput[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	for _, key := range []string{"projectPath", "project_path"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mcpRemoteArtifactCommandFromHookObject(obj map[string]any, toolInput map[string]any) string {
	tool := strings.ToLower(toolNameFromHookObject(obj))
	if tool == "" || (!strings.Contains(tool, "issue") && !strings.Contains(tool, "merge") && !strings.Contains(tool, "pull") && !strings.Contains(tool, "_mr") && !strings.Contains(tool, "_pr")) {
		return ""
	}
	toolInput = mergeMCPToolFlags(toolInput)
	cli := ""
	switch {
	case strings.Contains(tool, "gitlab") || strings.Contains(tool, "glab"):
		cli = "glab"
	case strings.Contains(tool, "github") || strings.Contains(tool, "gh"):
		cli = "gh"
	default:
		return ""
	}
	kind := ""
	switch {
	case strings.Contains(tool, "merge_request") || strings.Contains(tool, "merge-request") || strings.Contains(tool, "_mr") || strings.HasSuffix(tool, "mr"):
		kind = "mr"
	case strings.Contains(tool, "pull_request") || strings.Contains(tool, "pull-request") || strings.Contains(tool, "_pr") || strings.HasSuffix(tool, "pr"):
		kind = "pr"
	case strings.Contains(tool, "issue"):
		kind = "issue"
	default:
		return ""
	}
	action := ""
	switch {
	case strings.HasSuffix(tool, "_for") || strings.Contains(tool, "create_for") || strings.Contains(tool, "create-for"):
		action = "for"
	case strings.Contains(tool, "create") || strings.Contains(tool, "open"):
		action = "create"
	case strings.Contains(tool, "update"):
		action = "update"
	case strings.Contains(tool, "edit"):
		action = "edit"
	default:
		return ""
	}
	var args []string
	args = append(args, cli, kind, action)
	for _, positional := range stringListValue(toolInput, "args", "arguments") {
		args = append(args, shellQuoteArg(positional))
	}
	if action == "for" && len(stringListValue(toolInput, "args", "arguments")) == 0 {
		for _, issue := range stringListValue(toolInput, "issue", "issue_iid", "issueIid") {
			args = append(args, shellQuoteArg(issue))
		}
	}
	if title := firstStringValue(toolInput, "title", "name", "subject"); title != "" {
		args = append(args, "--title", shellQuoteArg(title))
	}
	if body := firstStringValue(toolInput, "body", "description", "content", "markdown"); body != "" {
		if cli == "glab" {
			args = append(args, "--description", shellQuoteArg(body))
		} else {
			args = append(args, "--body", shellQuoteArg(body))
		}
	}
	for _, label := range stringListValue(toolInput, "label", "labels", "add_label", "add_labels") {
		args = append(args, "--label", shellQuoteArg(label))
	}
	if boolValue(toolInput, "copy_issue_labels", "copyIssueLabels", "copy_labels", "copyLabels") {
		args = append(args, "--copy-issue-labels")
	}
	if boolValue(toolInput, "with_labels", "withLabels") {
		args = append(args, "--with-labels")
	}
	if relatedIssue := firstStringValue(toolInput, "related_issue", "relatedIssue", "issue", "issue_iid", "issueIid"); relatedIssue != "" && cli == "glab" && kind == "mr" && action != "for" {
		args = append(args, "--related-issue", shellQuoteArg(relatedIssue))
	}
	for _, assignee := range stringListValue(toolInput, "assignee", "assignees", "add_assignee", "add_assignees") {
		args = append(args, "--assignee", shellQuoteArg(assignee))
	}
	for _, assigneeID := range stringListValue(toolInput, "assignee_id", "assignee_ids", "assigneeId", "assigneeIds") {
		args = append(args, "--assignee-id", shellQuoteArg(assigneeID))
	}
	return strings.Join(args, " ")
}

func mergeMCPToolFlags(toolInput map[string]any) map[string]any {
	flags, ok := toolInput["flags"].(map[string]any)
	if !ok || len(flags) == 0 {
		return toolInput
	}
	merged := make(map[string]any, len(flags)+len(toolInput))
	for key, value := range flags {
		merged[key] = value
	}
	for key, value := range toolInput {
		if key != "flags" {
			merged[key] = value
		}
	}
	return merged
}

func toolNameFromHookObject(obj map[string]any) string {
	for _, key := range []string{"tool_name", "tool", "name"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolValue(values map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := values[key].(bool); ok && value {
			return true
		}
	}
	return false
}

func stringListValue(values map[string]any, keys ...string) []string {
	var out []string
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			for _, part := range strings.Split(value, ",") {
				if part = strings.TrimSpace(part); part != "" {
					out = append(out, part)
				}
			}
		case []any:
			for _, item := range value {
				switch item := item.(type) {
				case string:
					if strings.TrimSpace(item) != "" {
						out = append(out, strings.TrimSpace(item))
					}
				case float64:
					out = append(out, strconv.FormatFloat(item, 'f', -1, 64))
				case int:
					out = append(out, strconv.Itoa(item))
				}
			}
		case []string:
			for _, item := range value {
				if strings.TrimSpace(item) != "" {
					out = append(out, strings.TrimSpace(item))
				}
			}
		case float64:
			out = append(out, strconv.FormatFloat(value, 'f', -1, 64))
		case int:
			out = append(out, strconv.Itoa(value))
		}
		if len(out) > 0 {
			return out
		}
	}
	return out
}

func shellQuoteArg(value string) string {
	return strconv.Quote(value)
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

func hookInputBool(input []byte, key string) bool {
	if v, ok := hookInputObject(input)[key].(bool); ok {
		return v
	}
	return false
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
