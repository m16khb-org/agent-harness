package hookprompt

import (
	"encoding/json"
	"strings"
)

func FromHookInput(input []byte) string {
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

func IsStopContinuation(prompt string) bool {
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
	if strings.Contains(trimmed, "자동진행하지") {
		return true
	}
	if strings.Contains(lower, "no-auto-proceed") {
		return true
	}
	return strings.Contains(trimmed, "다음 행동 판단 지점에 도달했습니다") &&
		strings.Contains(trimmed, "훅이 관찰한 근거")
}

// ShouldConsumeNextActionRelay reports whether a user prompt consumes the
// pending Stop next-action relay record. Any real user prompt makes the
// previously presented choices obsolete, while synthetic continuation prompts
// keep the record because the user never answered them.
func ShouldConsumeNextActionRelay(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" || IsStopContinuation(trimmed) {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "active goal") ||
		strings.Contains(lower, "goal continuation") ||
		strings.Contains(lower, "no-auto-proceed judgement") ||
		strings.Contains(lower, "without an explicit user choice") {
		return false
	}
	return true
}

func hasNextActionSection(prompt string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(prompt, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "선택지:") ||
			strings.HasPrefix(lower, "options:") ||
			strings.HasPrefix(lower, "next actions:") {
			return true
		}
	}
	return false
}
