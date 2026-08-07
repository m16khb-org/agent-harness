package hookprompt_test

import (
	"strings"
	"testing"

	core "agent-harness/internal/adapter/core"
	"agent-harness/internal/core/hookprompt"
)

func TestKarpathyFirstFiresOnSubstantivePrompts(t *testing.T) {
	// Default-on: any prompt with fresh intent fires, even without keyword
	// hints or repo state, and the user notice is set so it never fires
	// silently.
	for _, prompt := range []string{
		"로그인 세션이 가끔 끊기는데 원인 좀 봐줘",
		"안녕하세요, 오늘 날씨 어때요?",
		"3개의 버그를 우선순위대로 고쳐줘",
	} {
		got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: prompt})
		if !got.KarpathyFirst || !got.ShouldInject {
			t.Fatalf("expected karpathy-first to fire for %q: %+v", prompt, got)
		}
		if !strings.Contains(got.AdditionalContext, "- karpathy-first: ") ||
			!strings.Contains(got.AdditionalContext, "증강된 요청:") ||
			!strings.Contains(got.AdditionalContext, "증강 불필요") {
			t.Fatalf("karpathy-first directive missing for %q:\n%s", prompt, got.AdditionalContext)
		}
		if got.UserNotice != hookprompt.KarpathyFirstUserNotice {
			t.Fatalf("user notice must accompany karpathy-first for %q: %+v", prompt, got)
		}
	}
}

func TestKarpathyFirstSkipsPromptsWithoutFreshIntent(t *testing.T) {
	// Slash commands, next-action choice replies, and bare acknowledgements
	// respond to existing plans; there is nothing to augment.
	for _, prompt := range []string{
		"/commit-push",
		"1",
		"2번",
		"3.",
		"1번 진행해줘",
		"추천대로",
		"네",
		"계속 진행",
		"go ahead",
	} {
		got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: prompt})
		if got.KarpathyFirst || got.UserNotice != "" {
			t.Fatalf("karpathy-first must not fire for %q: %+v", prompt, got)
		}
		if strings.Contains(got.AdditionalContext, "karpathy-first") {
			t.Fatalf("directive leaked for %q:\n%s", prompt, got.AdditionalContext)
		}
	}
}

func TestKarpathyFirstOptOutPrefixSkipsButKeepsRouting(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "그대로: 변경사항 커밋하고 푸시해줘"})
	if got.KarpathyFirst || got.UserNotice != "" || strings.Contains(got.AdditionalContext, "karpathy-first") {
		t.Fatalf("opt-out prefix must skip karpathy-first: %+v", got)
	}
	// The stripped prompt must still drive keyword routing.
	found := false
	for _, hint := range got.Hints {
		if hint.Tool == "atomic-commit-push" {
			found = true
		}
	}
	if !found {
		t.Fatalf("opt-out prefix must not pollute keyword routing: %+v", got.Hints)
	}
}

func TestKarpathyFirstDisableSwitchSkipsButKeepsRouting(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{
		Prompt:               "변경사항 커밋하고 푸시해줘",
		DisableKarpathyFirst: true,
	})
	if got.KarpathyFirst || got.UserNotice != "" || strings.Contains(got.AdditionalContext, "karpathy-first") {
		t.Fatalf("disable switch must skip karpathy-first: %+v", got)
	}
	found := false
	for _, hint := range got.Hints {
		if hint.Tool == "atomic-commit-push" {
			found = true
		}
	}
	if !found {
		t.Fatalf("disable switch must not disable normal routing: %+v", got.Hints)
	}
}

func TestKarpathyFirstOptOutPrefixAloneDoesNotInject(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "그대로:"})
	if got.ShouldInject || got.AdditionalContext != "" {
		t.Fatalf("prefix-only prompt must not inject: %+v", got)
	}
}

func recordChoicesRelayForTest(t *testing.T, repo string) {
	t.Helper()
	trigger := core.BuildNextActionJudgementTrigger("작업 완료.\n\n선택지:\n1. 킬스위치를 구현하고 테스트까지 완료 (추천)\n2. 계획 문서만 커밋\n3. 보류하고 관찰 지속")
	if relay := core.RecordStopNextActionRelay(repo, trigger); !relay.ShouldRelay {
		t.Fatalf("expected relay record to be written: %+v", relay)
	}
}

func TestKarpathyFirstExpandsChoiceReplyFromRelay(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	recordChoicesRelayForTest(t, repo)
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "2번", Repo: repo})
	if !got.KarpathyFirst || !got.ShouldInject {
		t.Fatalf("choice reply with relay record must fire expansion: %+v", got)
	}
	for _, want := range []string{"선택지 2번을 선택했다", "계획 문서만 커밋", "증강된 요청:"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("choice expansion missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	if !strings.Contains(got.UserNotice, "선택지 2번") {
		t.Fatalf("choice expansion notice must name the selection: %+v", got)
	}
}

func TestKarpathyFirstExpandsRecommendedReplyFromRelay(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	recordChoicesRelayForTest(t, repo)
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "추천대로", Repo: repo})
	if !got.KarpathyFirst || !strings.Contains(got.AdditionalContext, "킬스위치를 구현하고") {
		t.Fatalf("recommended reply must expand to the recommended option: %+v", got)
	}
}

func TestKarpathyFirstChoiceExpansionStaysQuietWithoutRelay(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "2번", Repo: repo})
	if got.KarpathyFirst || got.UserNotice != "" || strings.Contains(got.AdditionalContext, "karpathy-first") {
		t.Fatalf("choice reply without a relay record must stay unaugmented: %+v", got)
	}
}

func TestKarpathyFirstChoiceExpansionSkipsOutOfRangeAndDisabled(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	recordChoicesRelayForTest(t, repo)
	outOfRange := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "9번", Repo: repo})
	if outOfRange.KarpathyFirst {
		t.Fatalf("out-of-range choice must not expand: %+v", outOfRange)
	}
	disabled := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "2번", Repo: repo, DisableKarpathyFirst: true})
	if disabled.KarpathyFirst || strings.Contains(disabled.AdditionalContext, "karpathy-first") {
		t.Fatalf("disable switch must also gate choice expansion: %+v", disabled)
	}
}

func TestKarpathyRuleExpandedDispatchKeywords(t *testing.T) {
	// The dispatch-hardening hint (rules.go) must also fire on common
	// implementation verbs, not only explicit prompt-engineering wording.
	for _, prompt := range []string{
		"이 기능 구현해줘",
		"결제 모듈 만들어줘",
		"이 코드 리팩토링 해줘",
		"refactor this package for clarity",
	} {
		if !hintToolsFor(t, prompt)["karpathy"] {
			t.Fatalf("prompt %q must surface the karpathy dispatch hint", prompt)
		}
	}
}
