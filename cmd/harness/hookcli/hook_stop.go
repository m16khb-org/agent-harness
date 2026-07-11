package hookcli

import (
	"encoding/json"
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
	enforceEngelbartCanvasSections := fs.Bool("enforce-engelbart-canvas-sections", false, "block Stop when Engelbart meeting Canvas output lacks required appendix/transcript sections")
	relayNextActionJudgement := fs.Bool("relay-next-action-judgement", false, "re-enter the main agent when the final response contains inspectable next-action facts")
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
	payloadHost := strings.TrimSpace(hookinput.HostFromHookInput(stdin))
	flagHost := strings.TrimSpace(*host)
	hostConflict := flagHost != "" && payloadHost != "" && !strings.EqualFold(flagHost, payloadHost)
	resolvedHost := strings.ToLower(flagHost)
	if resolvedHost == "" {
		resolvedHost = strings.ToLower(payloadHost)
	}
	if resolvedHost == "" {
		resolvedHost = string(hookadapter.HostCodex)
	}
	suppressNextAction := !hostConflict && core.SuppressStopNextActionForCompletedWorker(core.HookToolUseLifecycleRequest{
		Repo:      parsedRepo,
		CWD:       hookinput.CWDFromHookInput(stdin),
		Host:      resolvedHost,
		SessionID: hookinput.SessionIDFromHookInput(stdin),
		AgentID:   hookinput.AgentIDFromHookInput(stdin),
	})
	result := core.BuildLifecycleStopReminder(parsedRepo)
	message := hookinput.LastAssistantMessageFromHookInput(stdin)
	if message == "" {
		message = hookinput.ReadLastAssistantMessageFromTranscript(hookinput.TranscriptPathFromHookInput(stdin))
	}
	transcriptPath := hookinput.TranscriptPathFromHookInput(stdin)
	nextActions := core.BuildNumberedNextActionsDecision(
		message,
		*enforceNumberedNextActions,
		"stop",
	)
	stopHookActive := hookinput.Bool(stdin, "stop_hook_active")
	nextActionTriggerEnabled := *relayNextActionJudgement
	nextActionTrigger := core.BuildNextActionJudgementTrigger(message)
	noAutoProceedJudgement := core.IsNoAutoProceedJudgement(message)
	engelbartCanvasBlock, engelbartCanvasReason := buildEngelbartCanvasSectionsBlock(message, transcriptPath, *enforceEngelbartCanvasSections)
	if *jsonOut {
		return printJSON(map[string]any{
			"lifecycle":                    result,
			"numbered_next_actions":        nextActions,
			"next_action_judgement":        nextActionTrigger,
			"next_action_judgement_active": nextActionTriggerEnabled,
			"no_auto_proceed_judgement":    noAutoProceedJudgement,
			"engelbart_canvas_sections": map[string]any{
				"decision": map[bool]string{true: "block", false: "allow"}[engelbartCanvasBlock],
				"reason":   engelbartCanvasReason,
			},
		})
	}
	ho := hookadapter.Resolve(strings.TrimSpace(*host))
	if nextActionTriggerEnabled && nextActionTrigger.ShouldReenterAgent && !suppressNextAction {
		relayRecord := core.RecordStopNextActionRelay(parsedRepo, nextActionTrigger)
		if !relayRecord.ShouldRelay {
			return printJSON(ho.FormatNoop())
		}
		reason := core.BuildNextActionJudgementRelayReason(nextActionTrigger)
		if facts := core.StopOrchestrationRelayFacts(parsedRepo); facts != "" {
			reason += " 관찰된 orchestration 상태: " + facts + "."
		}
		return printJSON(ho.FormatStopBlock(reason))
	}
	// Guard the Engelbart canvas block with stop_hook_active, mirroring the
	// missing-choice gate below: hosts set it true when this Stop is itself a
	// continuation of a prior stop-hook block, so re-blocking a re-entered turn
	// would loop forever even after the response recovered.
	if engelbartCanvasBlock && !stopHookActive {
		markHookMetricBlocked()
		return printJSON(ho.FormatStopBlock(engelbartCanvasReason))
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
	if nextActions.Decision == "block" && !stopHookActive && !noAutoProceedJudgement && !suppressNextAction {
		markHookMetricBlocked()
		return printJSON(ho.FormatStopBlock(nextActions.Reason))
	}
	// Codex and Claude Stop hooks only accept the stop-control schema
	// (for example decision/reason/systemMessage) or an empty object. Unlike
	// prompt/compact hooks, Stop cannot inject additionalContext; returning
	// hookSpecificOutput makes Codex report "invalid stop hook JSON output". Keep the
	// raw reminder available behind --json, but emit a no-op host payload here.
	return printJSON(ho.FormatNoop())
}

func buildEngelbartCanvasSectionsBlock(message, transcriptPath string, enforce bool) (bool, string) {
	if !enforce {
		return false, ""
	}
	// Scope evidence to the most recent canvas write instead of joining the whole
	// transcript: a stale earlier-incomplete canvas must not block a now-complete
	// one, and a corrected second create_canvas must not trip the cross-blob section
	// order check.
	write, hasWrite := latestSlackCanvasWrite(transcriptPath)
	evidence := ""
	createContext := false
	if hasWrite {
		evidence = write.Content
		createContext = write.IsCreate
	}
	if strings.TrimSpace(evidence) == "" {
		if !looksLikeEngelbartCanvasCreationContext(message) {
			return false, ""
		}
		evidence = message
		createContext = true
	}
	if !looksLikeEngelbartCanvasContext(message + "\n" + evidence) {
		return false, ""
	}
	// Only a freshly created canvas must carry the full template. A
	// slack_update_canvas is an incremental fix, so it can clear the gate when it is
	// the most recent write (the canvas was corrected) but never blocks on its own
	// (a partial append must not be forced to repeat the whole template).
	if !createContext {
		return false, ""
	}
	if len(missingRequiredEngelbartCanvasBlocks(evidence)) == 0 || len(missingRequiredEngelbartCanvasBlocks(message)) == 0 {
		return false, ""
	}
	missing := missingRequiredEngelbartCanvasBlocks(evidence)
	if len(missing) == 0 {
		missing = missingRequiredEngelbartCanvasBlocks(message)
	}
	return true, "Stop hook blocked because Engelbart meeting Canvas creation is missing required template blocks: " + strings.Join(missing, ", ") + ". Create the Canvas with the full Engelbart template before finalizing; do not ask the user to re-request this."
}

func looksLikeEngelbartCanvasContext(text string) bool {
	lower := strings.ToLower(text)
	hasCanvas := strings.Contains(lower, "canvas") || strings.Contains(text, "캔버스") || strings.Contains(lower, "bubbletap.slack.com/docs")
	hasMeeting := strings.Contains(text, "회의록") || strings.Contains(text, "회의") || strings.Contains(text, "전사") || strings.Contains(lower, "clova") || strings.Contains(lower, "engelbart")
	return hasCanvas && hasMeeting
}

func looksLikeEngelbartCanvasCreationContext(text string) bool {
	if !looksLikeEngelbartCanvasContext(text) {
		return false
	}
	lower := strings.ToLower(text)
	return strings.Contains(text, "생성") ||
		strings.Contains(text, "만들") ||
		strings.Contains(text, "새 Canvas") ||
		strings.Contains(text, "새 캔버스") ||
		strings.Contains(lower, "created") ||
		strings.Contains(lower, "create_canvas")
}

type requiredEngelbartBlock struct {
	Label        string
	Alternatives []string
}

func missingRequiredEngelbartCanvasBlocks(text string) []string {
	blocks := []requiredEngelbartBlock{
		{Label: "top status block", Alternatives: []string{"::: {.callout}", "> 회의일"}},
		{Label: "## 메타데이터", Alternatives: []string{"## 메타데이터"}},
		{Label: "metadata table", Alternatives: []string{"|Field|Value|", "| Field | Value |", "|Field | Value|", "| Field|Value |"}},
		{Label: "## TL;DR", Alternatives: []string{"## TL;DR"}},
		{Label: "## 결정사항", Alternatives: []string{"## 결정사항"}},
		{Label: "## 액션 보드", Alternatives: []string{"## 액션 보드"}},
		{Label: "## 주제별 논의", Alternatives: []string{"## 주제별 논의"}},
		{Label: "## 후속 확인", Alternatives: []string{"## 후속 확인"}},
		{Label: "## 리스크/열린 질문", Alternatives: []string{"## 리스크/열린 질문", "## 리스크 / 열린 질문"}},
		{Label: "appendix divider", Alternatives: []string{"\n---\n", "\r\n---\r\n"}},
		{Label: "## 보정 및 원문 부록", Alternatives: []string{"## 보정 및 원문 부록"}},
		{Label: "### 용어 보정", Alternatives: []string{"### 용어 보정"}},
		{Label: "### 불확실 단어/문장 보정", Alternatives: []string{"### 불확실 단어/문장 보정"}},
		{Label: "### 참석자/화자 보정", Alternatives: []string{"### 참석자/화자 보정"}},
		{Label: "### 원문 전사본 전문", Alternatives: []string{"### 원문 전사본 전문"}},
		{Label: "원문 text 코드블록", Alternatives: []string{"```text"}},
	}
	missing := []string{}
	lastIndex := -1
	for _, block := range blocks {
		index := indexOfAny(text, block.Alternatives)
		if index < 0 {
			missing = append(missing, block.Label)
			continue
		}
		if index < lastIndex {
			missing = append(missing, block.Label+"(order)")
			continue
		}
		lastIndex = index
	}
	return missing
}

func indexOfAny(text string, alternatives []string) int {
	best := -1
	for _, alternative := range alternatives {
		index := strings.Index(text, alternative)
		if index < 0 {
			continue
		}
		if best < 0 || index < best {
			best = index
		}
	}
	return best
}

// slackCanvasWrite is one Slack canvas tool result captured from the transcript.
type slackCanvasWrite struct {
	Content  string
	IsCreate bool // true for slack_create_canvas, false for slack_update_canvas
}

// slackCanvasToolKind distinguishes the canvas tool context while walking a
// transcript line so nested content is attributed to the right write.
type slackCanvasToolKind int

const (
	slackCanvasToolNone slackCanvasToolKind = iota
	slackCanvasToolCreate
	slackCanvasToolUpdate
)

// latestSlackCanvasWrite returns the most recent Slack canvas write (create or
// update) recorded in the transcript. Scoping to the single most-recent canvas
// keeps stale earlier-incomplete content from blocking a now-complete canvas and
// lets a slack_update_canvas fix clear the gate.
func latestSlackCanvasWrite(transcriptPath string) (slackCanvasWrite, bool) {
	transcriptPath = strings.TrimSpace(transcriptPath)
	if transcriptPath == "" {
		return slackCanvasWrite{}, false
	}
	b, err := os.ReadFile(transcriptPath)
	if err != nil {
		return slackCanvasWrite{}, false
	}
	var latest slackCanvasWrite
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			continue
		}
		for _, write := range collectCanvasWriteValues(value, slackCanvasToolNone) {
			latest = write
			found = true
		}
	}
	return latest, found
}

func collectCanvasWriteValues(value any, kind slackCanvasToolKind) []slackCanvasWrite {
	switch v := value.(type) {
	case map[string]any:
		if mapped := slackCanvasToolKindForMap(v); mapped != slackCanvasToolNone {
			kind = mapped
		}
		writes := []slackCanvasWrite{}
		for key, child := range v {
			if kind != slackCanvasToolNone && strings.EqualFold(key, "content") {
				if text, ok := child.(string); ok && strings.TrimSpace(text) != "" {
					writes = append(writes, slackCanvasWrite{Content: text, IsCreate: kind == slackCanvasToolCreate})
				}
			}
			writes = append(writes, collectCanvasWriteValues(child, kind)...)
			if text, ok := child.(string); ok {
				writes = append(writes, collectCanvasWriteValuesFromJSONString(text, kind)...)
			}
		}
		return writes
	case []any:
		writes := []slackCanvasWrite{}
		for _, child := range v {
			writes = append(writes, collectCanvasWriteValues(child, kind)...)
		}
		return writes
	case string:
		return collectCanvasWriteValuesFromJSONString(v, kind)
	default:
		return nil
	}
}

func collectCanvasWriteValuesFromJSONString(text string, kind slackCanvasToolKind) []slackCanvasWrite {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil
	}
	return collectCanvasWriteValues(parsed, kind)
}

func slackCanvasToolKindForMap(m map[string]any) slackCanvasToolKind {
	for _, key := range []string{"name", "tool_name", "recipient_name"} {
		value, ok := m[key].(string)
		if !ok {
			continue
		}
		if isSlackCanvasCreateTool(value) {
			return slackCanvasToolCreate
		}
		if isSlackCanvasUpdateTool(value) {
			return slackCanvasToolUpdate
		}
	}
	return slackCanvasToolNone
}

func isSlackCanvasCreateTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "slack_create_canvas")
}

func isSlackCanvasUpdateTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "slack_update_canvas")
}
