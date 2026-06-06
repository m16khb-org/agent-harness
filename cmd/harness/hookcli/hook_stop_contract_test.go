package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHookStopEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	out := captureStdoutForTest(t, func() {
		if err := runHookStop([]string{"--repo", repo}); err != nil {
			t.Fatalf("runHookStop: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("stop hook output is not JSON: %q: %v", out, err)
	}
	if len(obj) != 0 {
		t.Fatalf("Stop hook host output must be a no-op object, got %s", out)
	}
	if strings.Contains(out, "hookSpecificOutput") || strings.Contains(out, "additionalContext") {
		t.Fatalf("Stop hook output contains unsupported injection fields: %s", out)
	}
}

func TestRunHookStopBlocksMissingNumberedNextActionsWhenExpected(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"작업을 진행했습니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	// continue must stay true so the agent continues in-turn and presents the
	// choices; continue:false would hard-stop and surface the reason to the user.
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices with an in-turn continuation, got %+v", obj)
	}
}

func TestRunHookStopAllowsStopWhenStopHookActiveMissingChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"last_assistant_message":"작업을 진행했습니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("stop_hook_active must suppress the next-action block to avoid loops, got %+v", obj)
	}
}

func TestRunHookStopBlockReasonTellsAgentToPresentContextSpecificChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	message := "완료했습니다.\n\nCommit: 18172349f3 fix(dmm): use persona tags for ranking registration\nIssueOps phase: pr, strict readiness 통과"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+strings.ReplaceAll(message, "\n", "\\n")+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices with an in-turn continuation, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "caused the block") || !strings.Contains(reason, "context-specific") {
		t.Fatalf("expected Stop hook reason to tell the agent why it blocked and to create context-specific choices, got %q", reason)
	}
}

func TestRunHookStopAllowsNumberedNextActionsWhenExpected(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"선택지:\n1. 진행: 검증합니다. (추천)\n2. 축소 진행: 일부만 합니다.\n3. 보류: 멈춥니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("expected Stop hook to allow numbered choices with no-op output, got %+v", obj)
	}
}

func TestRunHookStopBlocksNumberedNextActionsWithoutOneRecommendation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	for _, message := range []string{
		"선택지:\\n1. 진행: 검증합니다.\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다.",
		"선택지:\\n1. 진행: 검증합니다. (추천)\\n2. 축소 진행: 일부만 합니다. (추천)\\n3. 보류: 멈춥니다.",
	} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+message+`"}`, func() error {
			return runHookStop([]string{"--enforce-numbered-next-actions"})
		})
		if obj["continue"] != true || obj["decision"] != "block" {
			t.Fatalf("expected malformed recommendations to block, got %+v", obj)
		}
	}
}

func TestRunHookStopDoesNotTreatNumberedExplanationAsAutoProceedChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := strings.Join([]string{
		"설명입니다.",
		"1. Stop hook이 먼저 정적 휴리스틱으로 자동진행 후보를 판정합니다.",
		"2. 메인 에이전트는 실행 여부만 판단합니다.",
		"3. `agent-harness`가 추천 선택지를 분석해서 자동진행 후보라고 판단합니다.",
	}, "\\n")
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--relay-next-action-judgement"})
	})
	reason, _ := obj["reason"].(string)
	if obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices, got %+v", obj)
	}
	if !strings.Contains(reason, "lacks well-formed numbered next actions") {
		t.Fatalf("expected missing-next-actions block, got %+v", obj)
	}
	if strings.Contains(reason, "자동진행 후보") {
		t.Fatalf("numbered explanatory text must not be treated as auto-proceed choices, got %q", reason)
	}
}

func TestRunHookStopDoesNotAutoProceedDestructiveCleanup(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 정리 진행: merged worktree와 branch를 삭제합니다. (추천)\\n2. 보류: 유지합니다.\\n3. 확장 정리: 전체를 점검합니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	reason, _ := obj["reason"].(string)
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected facts relay block for destructive-looking text, got %+v", obj)
	}
	if !strings.Contains(reason, "판단 지점") || !strings.Contains(reason, "추천 선택지") {
		t.Fatalf("expected factual trigger reason, got %+v", obj)
	}
	for _, banned := range []string{"destructive", "irreversible", "점수", "임계값", "자동 진행 미적용"} {
		if strings.Contains(reason, banned) {
			t.Fatalf("Stop hook must not judge destructive-looking text with %q: %+v", banned, obj)
		}
	}
}

func TestRunHookStopDoesNotRelayNextActionJudgementWithoutFlag(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	// The relay flag is the on/off switch. Even with a valid recommended choice
	// present, omitting it must not re-enter the main agent.
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("next-action judgement relay must require its flag; got %+v", obj)
	}
}

func TestRunHookStopReadsLastAssistantMessageFromTranscript(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"작업했습니다."}]}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to inspect transcript and block with an in-turn continuation, got %+v", obj)
	}
}

func TestRunHookStopIgnoresSystemTranscriptObjectWithAssistantText(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := `{"type":"system","text":"assistant reminder without a final assistant response"}` + "\r\n"
	if err := os.WriteFile(transcript, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--json"})
	})
	next, _ := obj["numbered_next_actions"].(map[string]any)
	if next["decision"] != "allow" || next["reason"] != "no assistant message available to inspect" {
		t.Fatalf("expected system transcript object to be ignored, got %+v", obj)
	}
}
