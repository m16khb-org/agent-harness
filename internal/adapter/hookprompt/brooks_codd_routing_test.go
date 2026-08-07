package hookprompt_test

import "testing"

// Fix #11: a direct devil's-advocate / over-engineering / plan-review request
// must surface the brooks hint (sub-agent-only) instead of mis-routing to ADR
// recording. Fix #16: concurrency wording must surface the codd hint now that
// its keywords cover deadlock/lock/isolation/transaction terms. Same assertion
// style as TestPioneerSkillRoutingHintsFireOnMatchingPrompts.
func TestBrooksAndCoddRoutingHintsFireOnMatchingPrompts(t *testing.T) {
	cases := []struct {
		prompt string
		skill  string
	}{
		{"play devil's advocate on my plan", "brooks"},
		{"is this over-engineered?", "brooks"},
		{"이 설계가 과설계인지 봐줘", "brooks"},
		{"내 계획 검토 좀 해줘", "brooks"},
		{"debug this deadlock", "codd"},
		{"이 트랜잭션 락 경합 좀 봐줘", "codd"},
		{"raise the isolation level for this transaction", "codd"},
	}
	for _, tc := range cases {
		t.Run(tc.prompt, func(t *testing.T) {
			if !hintToolsFor(t, tc.prompt)[tc.skill] {
				t.Fatalf("prompt %q must surface the %s hint", tc.prompt, tc.skill)
			}
		})
	}
}

func TestBrooksRoutingHintStaysQuietOnUnrelatedPrompts(t *testing.T) {
	tools := hintToolsFor(t, "안녕하세요, 오늘 날씨 어때요?")
	if tools["brooks"] {
		t.Fatalf("unrelated prompt must not surface brooks")
	}
}
