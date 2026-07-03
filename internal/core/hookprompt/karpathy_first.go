package hookprompt

import (
	"fmt"
	"strings"
	"time"
)

// karpathyFirstDirectiveTail is the shared augment-then-act contract; the
// full method lives in the karpathy skill. Kept as one compact context line
// so the UserPromptSubmit payload stays transcript-friendly.
const karpathyFirstDirectiveTail = "응답 서두에 '증강된 요청:'으로 입출력 계약·핵심 제약·모호성 해소(가장 그럴듯한 해석 1개)를 반영한 증강 프롬프트를 표기하고 그것을 기준으로 진행. 원문과 실질 동일하면 '증강 불필요' 한 줄만 표기. 다단계·서브에이전트 작업은 karpathy 스킬을 로드해 정식 적용"

const karpathyFirstDirective = "원문을 바로 실행하지 말 것. " + karpathyFirstDirectiveTail

// KarpathyFirstUserNotice is shown to the user (Claude systemMessage) whenever
// the directive fires, so augmentation never happens silently.
const KarpathyFirstUserNotice = "🧪 karpathy-first — 증강된 프롬프트를 먼저 작성한 뒤 진행합니다 (건너뛰기: \"그대로:\" 접두사)"

// choiceRelayMaxAge guards against expanding a stale relay record left over
// from an old, unconsumed decision turn.
const choiceRelayMaxAge = 6 * time.Hour

func karpathyChoiceContextLine(index int, text string) string {
	return fmt.Sprintf("- karpathy-first: 사용자가 선택지 %d번을 선택했다: %q — 이 선택지를 원문 요청으로 삼아 %s", index, text, karpathyFirstDirectiveTail)
}

func karpathyChoiceUserNotice(index int) string {
	return fmt.Sprintf("🧪 karpathy-first — 선택지 %d번 내용을 복원해 증강합니다", index)
}

// resolveChoiceExpansion maps a bare choice reply ("1", "2번", "추천대로")
// back to the chosen option's full text using the Stop next-action relay
// record. Best-effort: without a fresh record it reports false and the choice
// reply stays unaugmented, matching the pre-existing behavior.
func resolveChoiceExpansion(prompt, repo string) (int, string, bool) {
	if strings.TrimSpace(repo) == "" {
		return 0, "", false
	}
	index, useRecommended, ok := choiceReplySelection(prompt)
	if !ok {
		return 0, "", false
	}
	record, found := ReadStopNextActionRelay(repo)
	if !found || relayRecordStale(record.UpdatedAt) {
		return 0, "", false
	}
	if useRecommended {
		index = record.RecommendedIndex
	}
	for _, candidate := range record.Candidates {
		if candidate.Index == index && strings.TrimSpace(candidate.Text) != "" {
			return index, strings.TrimSpace(candidate.Text), true
		}
	}
	return 0, "", false
}

// choiceReplySelection classifies a prompt as a choice reply and returns the
// selected index, or useRecommended for explicit "follow the recommendation"
// answers. Deliberately narrower than isChoiceReply: only replies that name a
// concrete selection can be expanded.
func choiceReplySelection(prompt string) (index int, useRecommended, ok bool) {
	trimmed := strings.TrimSpace(prompt)
	switch strings.ToLower(trimmed) {
	case "추천대로", "추천":
		return 0, true, true
	}
	if !isNumberedChoiceReply(trimmed) {
		return 0, false, false
	}
	runes := []rune(trimmed)
	digits := 0
	value := 0
	for digits < len(runes) && runes[digits] >= '0' && runes[digits] <= '9' {
		value = value*10 + int(runes[digits]-'0')
		digits++
	}
	if value <= 0 {
		return 0, false, false
	}
	return value, false, true
}

func relayRecordStale(updatedAt string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(updatedAt))
	if err != nil {
		return true
	}
	return time.Since(parsed) > choiceRelayMaxAge
}

// karpathyOptOutPrefix lets the user skip augmentation for one prompt.
const karpathyOptOutPrefix = "그대로:"

// karpathyFirstDecision reports whether the karpathy-first directive should
// fire for the prompt and returns the prompt with the opt-out prefix removed
// so downstream keyword routing is not polluted. The default is to fire;
// exclusions are limited to prompts that carry no fresh intent to augment.
func karpathyFirstDecision(prompt string) (bool, string) {
	trimmed := strings.TrimSpace(prompt)
	if rest, ok := strings.CutPrefix(trimmed, karpathyOptOutPrefix); ok {
		return false, strings.TrimSpace(rest)
	}
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return false, trimmed
	}
	if isChoiceReply(trimmed) {
		return false, trimmed
	}
	return true, trimmed
}

// isChoiceReply detects next-action choice answers and bare acknowledgements:
// they respond to an existing plan, so there is nothing to augment.
func isChoiceReply(prompt string) bool {
	if len([]rune(prompt)) <= 2 {
		return true
	}
	switch strings.ToLower(prompt) {
	case "추천대로", "그래", "계속", "계속 진행", "진행해", "진행해줘",
		"ok", "okay", "yes", "go ahead", "continue", "proceed":
		return true
	}
	return isNumberedChoiceReply(prompt)
}

// isNumberedChoiceReply matches short "3", "2번", "1. ", "1번 진행해줘" style
// answers while letting digit-led prompts with real content through.
func isNumberedChoiceReply(prompt string) bool {
	runes := []rune(prompt)
	digits := 0
	for digits < len(runes) && runes[digits] >= '0' && runes[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits > 2 {
		return false
	}
	rest := strings.TrimSpace(string(runes[digits:]))
	rest = strings.TrimPrefix(rest, "번")
	rest = strings.TrimPrefix(rest, ".")
	rest = strings.TrimSpace(rest)
	return len([]rune(rest)) <= 6
}
