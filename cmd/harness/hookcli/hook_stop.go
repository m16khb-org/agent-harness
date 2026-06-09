package hookcli

import (
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookinput"
	hookadapter "agent-harness/internal/adapter/hook"
	"agent-harness/internal/core"
)

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
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = ResolveTarget("")
	}
	result := core.BuildLifecycleStopReminder(parsedRepo)
	message := hookinput.LastAssistantMessageFromHookInput(stdin)
	if message == "" {
		message = hookinput.ReadLastAssistantMessageFromTranscript(hookinput.TranscriptPathFromHookInput(stdin))
	}
	nextActions := core.BuildNumberedNextActionsDecision(
		message,
		*enforceNumberedNextActions,
		"stop",
	)
	// --auto-proceed-next-actions is retained only as a compatibility alias. This
	// hook no longer auto-proceeds or judges choices; it detects that an explicit
	// next-action review point exists and relays observed facts back to the main agent.
	stopHookActive := hookinput.Bool(stdin, "stop_hook_active")
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
	ho := hookadapter.Resolve(strings.TrimSpace(*host))
	// The external-LLM gate (core.EvaluateNextActionAutoProceedLLM) is intentionally
	// not called here: a synchronous agy/Gemini call measured ~13-25s, which is
	// unusable inside a Stop hook's latency budget. The hook also does not replace
	// that LLM with a local scorer. It reports only that the response reached an
	// explicit next-action judgement point and sends the observed facts to the main
	// agent, which owns safety, reversibility, alignment, and proceed/ask judgement.
	if nextActionTriggerEnabled && nextActionTrigger.ShouldReenterAgent {
		relayRecord := core.RecordStopNextActionRelay(parsedRepo, nextActionTrigger)
		if !relayRecord.ShouldRelay {
			return printJSON(ho.FormatNoop())
		}
		return printJSON(ho.FormatStopBlock(core.BuildNextActionJudgementRelayReason(nextActionTrigger)))
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
	// Guard only the missing-choice recovery with stop_hook_active: hosts set it
	// true when this Stop is itself a continuation of a prior stop-hook block. Valid
	// next-action choices still need the judgement relay above; otherwise a
	// recovered response can present choices and then silently stop.
	if nextActions.Decision == "block" && !stopHookActive {
		return printJSON(ho.FormatStopBlock(nextActions.Reason))
	}
	// Codex and Claude Stop hooks only accept the stop-control schema
	// (for example decision/reason/systemMessage) or an empty object. Unlike
	// prompt/compact hooks, Stop cannot inject additionalContext; returning
	// hookSpecificOutput makes Codex report "invalid stop hook JSON output". Keep the
	// raw reminder available behind --json, but emit a no-op host payload here.
	return printJSON(ho.FormatNoop())
}
