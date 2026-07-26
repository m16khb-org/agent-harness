package orca

import "testing"

// 어휘의 출처는 Orca CLI다.
//
//	$ orca orchestration task-update --help
//	Notes:
//	  Valid --status values: pending, ready, dispatched, completed, failed, blocked.
//
// core/operationalhealth의 settledTaskStatus가 이 집합을 그대로 반영한다.
// 두 정의가 갈리면 abandon 게이트가 통과시킨 task를 분류기가 계속 finding으로
// 보고하게 된다(#136에서 실제로 그랬다).
func TestExecutionTerminalTaskStatusMatchesTheOrcaVocabulary(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		t.Run("terminal/"+status, func(t *testing.T) {
			if !executionTerminalTaskStatus(status) {
				t.Fatalf("%q means the work is over and must be terminal", status)
			}
		})
	}
	for _, status := range []string{"pending", "ready", "dispatched", "blocked"} {
		t.Run("live/"+status, func(t *testing.T) {
			if executionTerminalTaskStatus(status) {
				t.Fatalf("%q can still hold or acquire a worker; treating it as terminal lets abandon strand it", status)
			}
		})
	}
}

// Orca가 거부하는 값은 관측될 수 없다. 그것을 종결로 인정해도 방어가 되지
// 않으며, 어휘의 출처만 흐려진다.
func TestExecutionTerminalTaskStatusDropsStatusesOrcaRejects(t *testing.T) {
	for _, status := range []string{"complete", "cancelled", "canceled", "closed"} {
		t.Run(status, func(t *testing.T) {
			if executionTerminalTaskStatus(status) {
				t.Fatalf("Orca rejects %q on task-update; it must not be treated as terminal", status)
			}
		})
	}
}

// 모르는 값은 종결이 아니다 — worker를 붙들고 있을 수 있다고 보는 쪽이 안전하다.
func TestExecutionTerminalTaskStatusTreatsUnknownAsLive(t *testing.T) {
	for _, status := range []string{"", "  ", "running", "archived"} {
		if executionTerminalTaskStatus(status) {
			t.Fatalf("%q is outside the vocabulary and must not be treated as terminal", status)
		}
	}
}
