package hookcli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
		parsedRepo = repoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = ResolveTarget("")
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
	if nextActionTriggerEnabled && nextActionTrigger.ShouldReenterAgent {
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
	// Guard only the missing-choice recovery with stop_hook_active: hosts set it
	// true when this Stop is itself a continuation of a prior stop-hook block. Valid
	// next-action choices still need the judgement relay above; otherwise a
	// recovered response can present choices and then silently stop.
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
	return fmt.Sprintf("다음 행동 판단 지점에 도달했습니다. 훅이 관찰한 근거: 명시적 선택지 %d개, 추천 선택지 %s. 훅은 안전성, 가역성, 사용자 의도 정합성, 진행 여부를 판단하지 않습니다. 메인 에이전트가 현재 대화와 작업 맥락을 근거로 직접 판단하세요. 자동진행한다면 왜 안전하고 가역적이며 사용자 의도에 맞는지 답변에 명시하고 지금 실행하세요. 자동진행 결과 보고에도 `선택지:` 3개와 정확히 하나의 `(추천)`을 포함하세요. 자동진행하지 않는다면 왜 사용자 결정이 필요한지 또는 왜 후속 선택 지점인지 답변에 명시한 뒤 멈추세요.", trigger.ChoiceCount, recommended)
}
