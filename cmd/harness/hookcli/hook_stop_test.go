package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// incompleteEngelbartCanvasContent is a meeting Canvas missing the required
// follow-up/appendix/transcript blocks; a create_canvas carrying it must block.
const incompleteEngelbartCanvasContent = "::: {.callout}\n회의일 2026-06-26 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요\n:::\n\n## 메타데이터\n|Field|Value|\n|---|---|\n|Date|2026-06-26|\n\n## TL;DR\n- 요약\n\n## 결정사항\n- **결정**\n\n## 액션 보드\n- [ ] 담당: 작업\n\n## 주제별 논의\n### 주제\n- 정리\n\n## 리스크 / 열린 질문\n- 리스크"

// completeEngelbartCanvasContent carries every required template block in order.
func completeEngelbartCanvasContent() string {
	return joinLinesForTest(
		"::: {.callout}",
		"회의일 2026-06-26 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요",
		":::",
		"",
		"## 메타데이터",
		"|Field|Value|",
		"|---|---|",
		"|Date|2026-06-26|",
		"",
		"## TL;DR",
		"- 요약",
		"",
		"## 결정사항",
		"- **결정**",
		"",
		"## 액션 보드",
		"- [ ] 담당: 작업",
		"",
		"## 주제별 논의",
		"### 주제",
		"- 정리",
		"",
		"## 후속 확인",
		"- 확인",
		"",
		"## 리스크/열린 질문",
		"- **질문**",
		"",
		"---",
		"",
		"## 보정 및 원문 부록",
		"",
		"### 용어 보정",
		"- `오인식` -> `정확한 표현`. 근거: 사용자 보정. 신뢰도: 높음. 확인 방법: source.",
		"",
		"### 불확실 단어/문장 보정",
		"- 없음.",
		"",
		"### 참석자/화자 보정",
		"- `참석자 1` -> `김현호`. 근거: 사용자 제공. 신뢰도: 높음. 확인 방법: source.",
		"",
		"### 원문 전사본 전문",
		"```text",
		"참석자 1 00:00",
		"원문",
		"```",
	)
}

func joinLinesForTest(lines ...string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}

func writeCanvasToolLine(t *testing.T, tool, content string) map[string]any {
	t.Helper()
	return map[string]any{
		"type":           "tool_use",
		"name":           tool,
		"recipient_name": tool,
		"input": map[string]any{
			"title":   "2026-06-26 [배포] TC NCP 마이그레이션 회의",
			"content": content,
		},
	}
}

func writeTranscriptForTest(t *testing.T, lines ...map[string]any) string {
	t.Helper()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := []byte{}
	for _, line := range lines {
		b, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, b...)
		body = append(body, '\n')
	}
	if err := os.WriteFile(transcript, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return transcript
}

// TestRunHookStopDoesNotReBlockEngelbartCanvasWhenStopHookActive proves the
// Engelbart gate honors stop_hook_active. Without the guard an incomplete
// create_canvas blocks every Stop, including the continuation Stop the prior
// block triggered, looping forever even after the response recovered.
// Fails before the guard fix (decision=block), passes after (no-op).
func TestRunHookStopDoesNotReBlockEngelbartCanvasWhenStopHookActive(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := writeTranscriptForTest(t, writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_create_canvas", incompleteEngelbartCanvasContent))
	msg := "새 회의록 Canvas를 만들었습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("stop_hook_active must suppress the Engelbart canvas block to avoid loops, got %+v", obj)
	}
}

// TestRunHookStopClearsEngelbartGateWhenCanvasFixedViaUpdate proves a canvas
// corrected through slack_update_canvas clears the gate. An incomplete
// create_canvas blocks; a following slack_update_canvas that supplies the full
// template is the most recent canvas write and must be accepted as clearing
// evidence. Fails before the update-acceptance + most-recent-scoping fix
// (the stale incomplete create still blocks), passes after (no-op).
func TestRunHookStopClearsEngelbartGateWhenCanvasFixedViaUpdate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := writeTranscriptForTest(t,
		writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_create_canvas", incompleteEngelbartCanvasContent),
		writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_update_canvas", completeEngelbartCanvasContent()),
	)
	msg := "회의록 Canvas를 업데이트했습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("a slack_update_canvas fix with the required sections must clear the Engelbart gate, got %+v", obj)
	}
}
