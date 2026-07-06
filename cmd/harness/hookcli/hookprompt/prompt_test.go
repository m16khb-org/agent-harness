package hookprompt

import "testing"

func TestFromHookInputExtractsPromptFromKnownShapes(t *testing.T) {
	cases := map[string]string{
		`{"prompt":"  hello  "}`:               "hello",
		`{"user_prompt":"hi"}`:                 "hi",
		`{"message":"message"}`:                "message",
		`{"text":"text"}`:                      "text",
		`{"hook_input":{"prompt":" nested "}}`: "nested",
		`not json`:                             "not json",
		`{"prompt":"","hook_input":{"x":"y"}}`: "",
		`   `:                                  "",
	}
	for input, want := range cases {
		if got := FromHookInput([]byte(input)); got != want {
			t.Fatalf("FromHookInput(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStopContinuationDetection(t *testing.T) {
	stopPrompts := []string{
		`<hook_prompt hook_run_id="stop:1">blocked</hook_prompt>`,
		"Stop hook blocked feedback: choose",
		"자동진행하지 않음",
		"no-auto-proceed judgement",
		"다음 행동 판단 지점에 도달했습니다\n훅이 관찰한 근거",
	}
	for _, prompt := range stopPrompts {
		if !IsStopContinuation(prompt) {
			t.Fatalf("expected stop continuation for %q", prompt)
		}
	}
	if !hasNextActionSection("선택지:\n1. a") || !hasNextActionSection("Options:\n1. a") || !hasNextActionSection("Next actions:\n1. a") {
		t.Fatal("expected next action section detection")
	}
}

func TestShouldConsumeNextActionRelay(t *testing.T) {
	for prompt, want := range map[string]bool{
		"1":          true,
		"2번":         true,
		"버그 3개를 고쳐줘": true,
		"ㅎㅇ":         true,
		"":           false,
		`<hook_prompt hook_run_id="stop:1">blocked</hook_prompt>`: false,
		"Stop hook blocked feedback: choose":                      false,
		"Active goal: keep working":                               false,
		"goal continuation: resume the loop":                      false,
		"no-auto-proceed judgement was left; do not resume":       false,
		"do not resume without an explicit user choice":           false,
	} {
		if got := ShouldConsumeNextActionRelay(prompt); got != want {
			t.Fatalf("ShouldConsumeNextActionRelay(%q) = %v, want %v", prompt, got, want)
		}
	}
}
