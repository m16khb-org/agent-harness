package hookcli

import (
	"testing"
)

func TestExecutionV1StopDuplicateFingerprintReturnsNoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	message := "사용자 결정이 필요한 상태입니다.\\n\\n선택지:\\n1. (추천) 외부 담당자에게 문의하고 답변을 공유합니다.\\n2. 임시 조치를 검토합니다.\\n3. 작업을 보류합니다."
	input := `{"cwd":"` + repo + `","session_id":"fresh-session","last_assistant_message":"` + message + `"}`

	first := runHookCapture(t, input, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["decision"] != "block" {
		t.Fatalf("first Stop may relay one bounded decision event: %+v", first)
	}

	second := runHookCapture(t, input, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(second) != 0 {
		t.Fatalf("the identical Stop event must terminate as a no-op: %+v", second)
	}
}
