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
	return strings.Contains(trimmed, "다음 행동 판단 지점에 도달했습니다") &&
		strings.Contains(trimmed, "훅이 관찰한 근거")
}
