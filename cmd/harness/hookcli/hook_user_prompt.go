package hookcli

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookenv"
	"agent-harness/cmd/harness/hookcli/hookinput"
	"agent-harness/internal/core"
)

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
	repo := hookinput.RepoFromHookInput(stdin)
	if repo == "" {
		repo = ResolveTarget("")
	}
	if !isStopHookContinuationPrompt(prompt) {
		core.ClearStopNextActionRelay(repo)
	}
	result := core.BuildUserPromptMCPHints(core.HookUserPromptRequest{Prompt: prompt, Repo: repo, EnableAgyHints: *enableAgyHints || hookenv.Bool("HARNESS_ENABLE_AGY_HINTS")})
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
	if strings.HasPrefix(trimmed, `<hook_prompt `) &&
		strings.Contains(trimmed, `hook_run_id="stop:`) &&
		strings.Contains(trimmed, `</hook_prompt>`) {
		return true
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "stop hook") &&
		strings.Contains(lower, "blocked") &&
		strings.Contains(lower, "feedback:") {
		return true
	}
	return strings.Contains(trimmed, "다음 행동 판단 지점에 도달했습니다") &&
		strings.Contains(trimmed, "훅이 관찰한 근거")
}
