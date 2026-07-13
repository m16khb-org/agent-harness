package hookcli

import (
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookenv"
	"agent-harness/cmd/harness/hookcli/hookinput"
	"agent-harness/cmd/harness/hookcli/hookprompt"
	hookadapter "agent-harness/internal/adapter/hook"
	"agent-harness/internal/core"
)

func runHookUserPrompt(args []string) error {
	fs := flag.NewFlagSet("hook user-prompt", flag.ContinueOnError)
	promptFlag := fs.String("prompt", "", "user prompt text; defaults to hook stdin JSON prompt")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	enableLLMHints := fs.Bool("enable-llm-hints", false, "suggest host-agent second-pass review when the prompt fits")
	disableKarpathyFirst := fs.Bool("disable-karpathy-first", false, "disable the karpathy-first prompt augmentation directive for this hook invocation")
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
	payloadHost := strings.TrimSpace(hookinput.HostFromHookInput(stdin))
	flagHost := strings.TrimSpace(*hostFlag)
	nativeHost := firstNonEmptyHookValue(payloadHost, flagHost)
	if nativeHost == "" {
		nativeHost = string(hookadapter.HostCodex)
	} else if payloadHost != "" && flagHost != "" && !strings.EqualFold(payloadHost, flagHost) {
		nativeHost = "conflict"
	}
	result := core.BuildUserPromptMCPHints(core.HookUserPromptRequest{
		Prompt:               prompt,
		Repo:                 repo,
		Host:                 nativeHost,
		SessionID:            hookinput.SessionIDFromHookInput(stdin),
		EnableLLMHints:       *enableLLMHints || hookenv.Bool("HARNESS_ENABLE_LLM_HINTS"),
		DisableKarpathyFirst: *disableKarpathyFirst || hookenv.Bool("HARNESS_DISABLE_KARPATHY_FIRST"),
	})
	// Clear only after BuildUserPromptMCPHints: choice replies ("1", "2번")
	// read the relay record to expand the chosen option before it is consumed.
	if hookprompt.ShouldConsumeNextActionRelay(prompt) {
		core.ClearStopNextActionRelay(repo)
	}
	if *jsonOut {
		return printJSON(result)
	}
	// The stable project-doc catalog now ships via SessionStart/PostCompact, so
	// UserPromptSubmit only carries the small, dynamic per-turn hints. The only
	// user-visible line is the karpathy-first notice: augmentation must never
	// fire silently, and Claude/Reasonix carry it via systemMessage. Codex has
	// no separate systemMessage channel here (userView would replace the hint
	// context), so the notice is Claude/Reasonix-only.
	host := flagHost
	ho := hookadapter.Resolve(host)
	userView := ""
	switch hookadapter.Host(host) {
	case hookadapter.HostClaude, hookadapter.HostReasonix:
		userView = result.UserNotice
	}
	return printJSON(ho.FormatContext("UserPromptSubmit", result.AdditionalContext, userView))
}
