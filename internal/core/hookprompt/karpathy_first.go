package hookprompt

import "strings"

// karpathyFirstDirective is appended as one compact context line so the
// UserPromptSubmit payload stays transcript-friendly. The full method lives in
// the karpathy skill; this line only enforces the one-shot augment-then-act
// contract on the main agent.
const karpathyFirstDirective = "원문을 바로 실행하지 말 것. 응답 서두에 '증강된 요청:'으로 입출력 계약·핵심 제약·모호성 해소(가장 그럴듯한 해석 1개)를 반영한 증강 프롬프트를 표기하고 그것을 기준으로 진행. 원문과 실질 동일하면 '증강 불필요' 한 줄만 표기. 다단계·서브에이전트 작업은 karpathy 스킬을 로드해 정식 적용"

// KarpathyFirstUserNotice is shown to the user (Claude systemMessage) whenever
// the directive fires, so augmentation never happens silently.
const KarpathyFirstUserNotice = "🧪 karpathy-first — 증강된 프롬프트를 먼저 작성한 뒤 진행합니다 (건너뛰기: \"그대로:\" 접두사)"

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
