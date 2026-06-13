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

func TestStopContinuationAndExplicitInstructionDetection(t *testing.T) {
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
		if IsExplicitNextActionInstruction(prompt) {
			t.Fatalf("stop continuation should not be explicit instruction: %q", prompt)
		}
	}
	for _, prompt := range []string{"계속 진행", "please continue", "2번", "go ahead"} {
		if !IsExplicitNextActionInstruction(prompt) {
			t.Fatalf("expected explicit instruction for %q", prompt)
		}
	}
	for _, prompt := range []string{"", "active goal continuation", "without an explicit user choice"} {
		if IsExplicitNextActionInstruction(prompt) {
			t.Fatalf("unexpected explicit instruction for %q", prompt)
		}
	}
	if !hasNextActionSection("선택지:\n1. a") || !hasNextActionSection("Options:\n1. a") || !hasNextActionSection("Next actions:\n1. a") {
		t.Fatal("expected next action section detection")
	}
}
