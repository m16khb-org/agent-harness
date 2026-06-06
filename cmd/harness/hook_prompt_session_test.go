package main

import (
	"strings"
	"testing"
)

func TestRunHookUserPromptDropsCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"prompt":"x","cwd":"`+repo+`"}`, func() error { return runHookUserPrompt(nil) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("user-prompt must not carry a catalog systemMessage: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); strings.Contains(ctx, "project docs (read what's relevant):") || strings.Contains(ctx, "📚") {
		t.Fatalf("user-prompt must not inject the project-doc catalog: %q", ctx)
	}
}

func TestRunHookUserPromptAgyHintsAreOptIn(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	input := `{"prompt":"이 계획을 검토하고 개선점을 분석해줘","cwd":"` + repo + `"}`
	disabled := runHookCapture(t, input, func() error { return runHookUserPrompt(nil) })
	if strings.Contains(hookAdditionalContext(disabled), "agy -p") {
		t.Fatalf("agy hint should be disabled by default: %q", hookAdditionalContext(disabled))
	}
	enabled := runHookCapture(t, input, func() error { return runHookUserPrompt([]string{"--enable-agy-hints"}) })
	if !strings.Contains(hookAdditionalContext(enabled), "agy -p for LLM second-pass review") {
		t.Fatalf("agy hint should be enabled by flag: %q", hookAdditionalContext(enabled))
	}
}

func TestRunHookUserPromptRoutesVCSRemoteWorkToCLIFirstWithMCPFallback(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	input := `{"prompt":"GitLab MR 코멘트를 확인하고 이슈를 업데이트해줘","cwd":"` + repo + `"}`
	obj := runHookCapture(t, input, func() error { return runHookUserPrompt(nil) })
	ctx := hookAdditionalContext(obj)
	if !strings.Contains(ctx, "VCS remote work: use authenticated CLI first") {
		t.Fatalf("VCS prompt should include CLI-first guidance, got %q", ctx)
	}
	if !strings.Contains(ctx, "MCP fallback") || !strings.Contains(ctx, "do not print tokens") {
		t.Fatalf("VCS prompt should include MCP fallback and token hygiene guidance, got %q", ctx)
	}
}

func TestRunHookSessionStartInjectsCatalogClaude(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHookSessionStart(nil) })
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
		t.Fatalf("SessionStart should show the pretty catalog via systemMessage: %v", obj["systemMessage"])
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("SessionStart should inject the compact catalog additionalContext: %q", ctx)
	}
}

func TestRunHookSessionStartCodexOmitsSystemMessage(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHookSessionStart([]string{"--host", "codex"}) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("Codex SessionStart must omit systemMessage: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "• ARCHITECTURE.md") || strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("Codex SessionStart additionalContext should be the readable catalog view: %q", ctx)
	}
}

func TestRunHookSessionStartSkipsOnCompactSource(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"compact"}`, func() error { return runHookSessionStart(nil) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("compact-source SessionStart should not inject (PostCompact owns it): %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); ctx != "" {
		t.Fatalf("compact-source SessionStart should emit no additionalContext: %q", ctx)
	}
}

func TestRunHookPostCompactInjectsCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPostCompact(nil) })
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("PostCompact should re-inject the catalog after compaction: %q", ctx)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") {
		t.Fatalf("PostCompact (claude) should show the pretty catalog via systemMessage: %v", obj["systemMessage"])
	}
}

func TestRunHookPostCompactCodexEmitsCompatibleSchema(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPostCompact([]string{"--host", "codex"}) })
	if _, ok := obj["hookSpecificOutput"]; ok {
		t.Fatalf("Codex PostCompact must not emit hookSpecificOutput: %+v", obj)
	}
	if _, ok := obj["additionalContext"]; ok {
		t.Fatalf("Codex PostCompact must not emit additionalContext: %+v", obj)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
		t.Fatalf("Codex PostCompact should use supported systemMessage catalog: %v", obj["systemMessage"])
	}
}

func TestRunHookPreCompactEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPreCompact(nil) })
	if len(obj) != 0 {
		t.Fatalf("PreCompact hook host output must be a no-op object, got %+v", obj)
	}
}

func TestRunHookPreToolUseEmitsCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"x.go"}}`, func() error { return runHookPreToolUse(nil) })
	if len(obj) != 0 {
		t.Fatalf("PreToolUse hook host output must be a no-op object, got %+v", obj)
	}
}

func TestRunHookPreToolUseRawJSONIsAllowByDefault(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"x.go"}}`, func() error {
		return runHookPreToolUse([]string{"--json"})
	})
	if obj["decision"] != "allow" || obj["source"] != "pre-tool-use" || obj["tool"] != "Edit" {
		t.Fatalf("unexpected PreToolUse raw result: %+v", obj)
	}
}

func TestRunHookUserPromptClearsSuppressedNextActionRelay(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	runHookCapture(t, `{"cwd":"`+repo+`","prompt":"계속 진행해"}`, func() error {
		return runHookUserPrompt(nil)
	})
	afterPrompt := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if afterPrompt["continue"] != true || afterPrompt["decision"] != "block" {
		t.Fatalf("expected UserPromptSubmit to clear relay suppression, got %+v", afterPrompt)
	}
}

func TestRunHookUserPromptDoesNotClearRelayForStopHookContinuationPrompt(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	hookPrompt := `<hook_prompt hook_run_id="stop:6:/Users/habin/.codex/hooks.json">다음 행동 판단 지점에 도달했습니다. 훅이 관찰한 근거를 메인 에이전트가 직접 판단하세요.</hook_prompt>`
	runHookCapture(t, `{"cwd":"`+repo+`","prompt":"`+hookPrompt+`"}`, func() error {
		return runHookUserPrompt(nil)
	})
	afterHookPrompt := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(afterHookPrompt) != 0 {
		t.Fatalf("Stop hook continuation prompt must not clear relay suppression, got %+v", afterHookPrompt)
	}
}

func TestRunHookUserPromptDoesNotClearRelayForClaudeStopFeedback(t *testing.T) {
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
	feedbackPrompt := strings.Join([]string{
		"Stop hook (blocked)",
		"feedback: 다음 행동 판단 지점에 도달했습니다. 훅이 관찰한 근거: 명시적 선택지 3개, 추천 선택지 1번.",
		"메인 에이전트가 현재 대화와 작업 맥락을 근거로 직접 판단하세요.",
	}, "\n")
	runHookCapture(t, `{"cwd":"`+repo+`","prompt":"`+feedbackPrompt+`"}`, func() error {
		return runHookUserPrompt(nil)
	})
	afterFeedback := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+secondMsg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(afterFeedback) != 0 {
		t.Fatalf("Claude Stop hook feedback prompt must not clear relay suppression, got %+v", afterFeedback)
	}
}
