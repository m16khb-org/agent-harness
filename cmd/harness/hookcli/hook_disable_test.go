package hookcli

import (
	"io"
	"os"
	"strings"
	"testing"
)

// runHookRawCapture는 runHookCapture와 달리 출력이 JSON이라고 가정하지 않는다.
// 비활성화된 훅은 아무것도 내보내지 않아야 하므로 원문 그대로 검사해야 한다.
func runHookRawCapture(t *testing.T, stdinJSON string, fn func() error) string {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() { _, _ = io.WriteString(w, stdinJSON); _ = w.Close() }()
	defer func() { os.Stdin = oldStdin }()
	return captureStdoutForTest(t, func() {
		if err := fn(); err != nil {
			t.Fatalf("hook: %v", err)
		}
	})
}

// 평소라면 Stop 훅이 결정 이벤트를 relay하며 block을 내보내는 입력이다
// (TestExecutionStopDuplicateFingerprintReturnsNoop과 동일한 계약).
func blockingStopHookInput(repo, session string) string {
	message := "사용자 결정이 필요한 상태입니다.\\n\\n선택지:\\n1. (추천) 외부 담당자에게 문의하고 답변을 공유합니다.\\n2. 임시 조치를 검토합니다.\\n3. 작업을 보류합니다."
	return `{"cwd":"` + repo + `","session_id":"` + session + `","last_assistant_message":"` + message + `"}`
}

// HARNESS_DISABLE_HOOKS는 agent-harness 밖의 저장소에서 하네스 훅을 끄기 위한
// kill-switch다. 켜져 있으면 어떤 훅 이벤트도 검사하지 않고 통과해야 한다.
func TestDisableHooksTurnsBlockingHookEventIntoNoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	t.Setenv("HARNESS_DISABLE_HOOKS", "1")
	repo := t.TempDir()

	out := runHookRawCapture(t, blockingStopHookInput(repo, "disabled-session"), func() error {
		return runHook([]string{"stop", "--relay-next-action-judgement"})
	})

	if strings.TrimSpace(out) != "" {
		t.Fatalf("훅이 비활성화되면 어떤 결정도 내보내지 않아야 한다: %q", out)
	}
}

// 대조군: 같은 입력이 kill-switch 없이는 실제로 block을 내보낸다. 이것이
// 깨지면 위 테스트는 훅 비활성화가 아니라 무해한 입력을 증명하는 셈이 된다.
func TestBlockingHookInputStillBlocksWithoutTheKillSwitch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	got := runHookCapture(t, blockingStopHookInput(repo, "enabled-session"), func() error {
		return runHook([]string{"stop", "--relay-next-action-judgement"})
	})

	if got["decision"] != "block" {
		t.Fatalf("대조군 입력은 kill-switch 없이 block이어야 한다: %+v", got)
	}
}

// failures/metrics는 훅 이벤트가 아니라 운영자가 훅 이력을 조회하는 창구다.
// 훅을 꺼둔 상태에서도 조회는 계속 되어야 한다 — 끄는 대상은 강제이지 관측이 아니다.
func TestDisableHooksKeepsOperatorQuerySubcommandsAvailable(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	t.Setenv("HARNESS_DISABLE_HOOKS", "1")

	for _, sub := range []string{"failures", "metrics"} {
		t.Run(sub, func(t *testing.T) {
			out := runHookRawCapture(t, "", func() error {
				return runHook([]string{sub, "--json"})
			})

			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s 조회는 훅 비활성화와 무관하게 응답해야 한다", sub)
			}
		})
	}
}
