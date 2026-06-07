package hookcli

import (
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookenv"
	"agent-harness/cmd/harness/hookcli/hookinput"
	"agent-harness/cmd/harness/hookcli/hookprompt"
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
		prompt = hookprompt.FromHookInput(stdin)
	}
	repo := hookinput.RepoFromHookInput(stdin)
	if repo == "" {
		repo = ResolveTarget("")
	}
	if !hookprompt.IsStopContinuation(prompt) {
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
