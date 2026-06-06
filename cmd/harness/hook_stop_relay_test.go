package main

import (
	"strings"
	"testing"
)

func TestRunHookStopRelaysRecommendedNextActionFactsToMainAgent(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 다음 테스트를 추가하고 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 검증합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to re-enter main agent with observed facts, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	for _, want := range []string{"판단 지점", "근거", "메인 에이전트", "추천 선택지", "자동진행", "자동진행하지", "결과 보고", "선택지"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("expected factual trigger directive containing %q, got %q", want, reason)
		}
	}
	for _, banned := range []string{"점수", "임계값", "자동진행 후보", "destructive", "eligible", "candidate", "되돌릴 수"} {
		if strings.Contains(reason, banned) {
			t.Fatalf("Stop hook reason must not include hook judgement wording %q: %q", banned, reason)
		}
	}
}

func TestRunHookStopRelaysSameNextActionChoicesOnlyOnce(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "사용자 결정이 필요한 상태입니다.\\n\\n선택지:\\n1. (추천) 사용자가 외부 담당자에게 문의문을 전달하고 답변을 공유한다.\\n2. 임시 조치 변경안을 검토하라고 지시한다.\\n3. 식별자 분리 설계안을 검토하라고 지시한다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	second := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(second) != 0 {
		t.Fatalf("duplicate next-action choices must not re-enter the agent, got %+v", second)
	}
}

func TestRunHookStopSuppressesRephrasedNextActionChoicesUntilProgress(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	firstMsg := "현재는 사용자 액션이 필요합니다.\\n\\n선택지:\\n1. (추천) 사용자가 FANZA 담당자에게 문의문을 전달하고 답변을 공유한다.\\n2. FANZA 답변 전 임시 변경안을 검토하라고 지시한다.\\n3. FANZA용 character_id 분리 설계를 검토하라고 지시한다."
	secondMsg := "같은 외부 확인 지점입니다.\\n\\n선택지:\\n1. (추천) 사용자가 담당자에게 문의문을 전달하고, 답변을 여기로 공유한다.\\n2. 임시 조치로 DELETE 중단 변경안을 검토하라고 지시한다.\\n3. 동일 id 복구 불가를 전제로 분리 설계안을 검토하라고 지시한다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+firstMsg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	second := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+secondMsg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(second) != 0 {
		t.Fatalf("rephrased unresolved next-action choices must not re-enter the agent before progress, got %+v", second)
	}
}

func TestRunHookPostToolUseClearsSuppressedNextActionRelay(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","command":"true"}`, func() error {
		return runHookPostToolUse(nil)
	})
	afterProgress := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if afterProgress["continue"] != true || afterProgress["decision"] != "block" {
		t.Fatalf("expected PostToolUse to clear relay suppression, got %+v", afterProgress)
	}
}

func TestRunHookStopKeepsAutoProceedFlagAsRelayAlias(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--auto-proceed-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("deprecated auto-proceed flag should remain a relay alias, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "판단 지점") || strings.Contains(reason, "점수") {
		t.Fatalf("expected alias to use factual relay wording, got %q", reason)
	}
}

func TestRunHookStopRelaysNextActionJudgementWhenStopHookActiveHasValidChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("stop_hook_active must not suppress valid next-action judgement relay, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "판단 지점") || !strings.Contains(reason, "메인 에이전트") {
		t.Fatalf("expected judgement relay reason while stop_hook_active is true, got %q", reason)
	}
}
