package operationalhealth

import "testing"

// 게이트와 분류기가 같은 어휘를 써야 한다. abandon의 orca 자원 게이트는
// adapter의 종결 판정(executionTerminalTaskStatus)에 기대는데, 분류기가 그중
// 일부를 모르면 게이트가 통과시킨 task를 분류기가 계속 finding으로 보고한다
// — 게이트의 목적이 잔여물 방지이므로 그 불일치는 게이트를 무의미하게 만든다(#136).
func TestTaskStatusVocabularyMatchesTheOrcaTerminalSet(t *testing.T) {
	// adapter의 executionTerminalTaskStatus가 종결로 보는 집합이다.
	for _, status := range []string{"completed", "complete", "failed", "cancelled", "canceled", "closed"} {
		t.Run(status, func(t *testing.T) {
			if !knownTaskStatus(status) {
				t.Fatalf("%q is a status Orca can report; classifying it as unknown hides a settled task", status)
			}
			if !settledTaskStatus(status) {
				t.Fatalf("%q cannot be dispatched and holds no worker; it must count as settled", status)
			}
		})
	}
}

// 실행 중인 상태는 여전히 종결이 아니다. 넓히는 방향이 활성 task까지
// 삼키면 실제 잔여물 검출력을 잃는다.
func TestLiveTaskStatusesStayUnsettled(t *testing.T) {
	for _, status := range []string{"ready", "dispatched"} {
		t.Run(status, func(t *testing.T) {
			if !knownTaskStatus(status) {
				t.Fatalf("%q must stay a known status", status)
			}
			if settledTaskStatus(status) {
				t.Fatalf("%q is still dispatchable and must not count as settled", status)
			}
		})
	}
}

// 어휘 밖의 값은 여전히 unknown이다. 모르는 상태를 조용히 종결로 다루면
// fail-open이 된다.
func TestUnrecognisedTaskStatusStaysUnknown(t *testing.T) {
	for _, status := range []string{"", "running", "paused", "archived"} {
		if knownTaskStatus(status) {
			t.Fatalf("%q is outside the Orca vocabulary and must stay unknown", status)
		}
	}
}
