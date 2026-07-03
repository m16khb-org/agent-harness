package hookprompt_test

import (
	"strings"
	"testing"

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
