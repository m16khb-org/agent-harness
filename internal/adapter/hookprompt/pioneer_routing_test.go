package hookprompt_test

import (
	hookpromptcontract "agent-harness/internal/contract/hookprompt"
	"testing"

	"agent-harness/internal/adapter/hookprompt"
)

func hintToolsFor(t *testing.T, prompt string) map[string]bool {
	t.Helper()
	got := hookprompt.BuildUserPromptMCPHints(hookpromptcontract.HookUserPromptRequest{Prompt: prompt})
	tools := map[string]bool{}
	for _, hint := range got.Hints {
		tools[hint.Tool] = true
	}
	return tools
}

// Non-issueops requests must route to the matching CS pioneer skill via
// keyword hints (issue #10): e.g. web-research wording surfaces berners-lee.
func TestPioneerSkillRoutingHintsFireOnMatchingPrompts(t *testing.T) {
	cases := []struct {
		prompt string
		skill  string
	}{
		{"웹에서 자료 조사해서 정리해줘", "berners-lee"},
		{"compare these libraries with web research and cite sources", "berners-lee"},
		{"테스트가 flaky한데 원인 찾아줘", "hopper"},
		{"diagnose why this regression happens", "hopper"},
		{"이 루프가 느려서 최적화하고 싶어", "dijkstra"},
		{"reduce the time complexity of this function", "dijkstra"},
		{"orders 테이블 인덱스 설계 좀 봐줘", "codd"},
		{"this query plan looks wrong, check the schema", "codd"},
		{"rebase 하다가 충돌 났어", "torvalds"},
		{"git bisect로 회귀 커밋 찾아줘", "torvalds"},
		{"변경사항 커밋하고 푸시해줘", "atomic-commit-push"},
		{"이 기능 구현 계획 세워줘", "von-neumann"},
		{"draft a decision-complete work plan", "von-neumann"},
		{"이 diff 슬롭 정리 전후 품질 측정해줘", "shannon"},
		{"이 프롬프트 최적화해줘", "karpathy"},
	}
	for _, tc := range cases {
		t.Run(tc.prompt, func(t *testing.T) {
			if !hintToolsFor(t, tc.prompt)[tc.skill] {
				t.Fatalf("prompt %q must surface the %s hint", tc.prompt, tc.skill)
			}
		})
	}
}

func TestPioneerSkillRoutingHintsStayQuietOnUnrelatedPrompts(t *testing.T) {
	tools := hintToolsFor(t, "안녕하세요, 오늘 날씨 어때요?")
	for _, skill := range []string{"berners-lee", "hopper", "dijkstra", "codd", "torvalds", "atomic-commit-push", "von-neumann", "shannon", "karpathy"} {
		if tools[skill] {
			t.Fatalf("unrelated prompt must not surface %s", skill)
		}
	}
}
