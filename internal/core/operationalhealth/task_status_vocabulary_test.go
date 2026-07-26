package operationalhealth

import "testing"

// 어휘의 출처는 orca CLI다.
//
//	$ orca orchestration task-update --help
//	Notes:
//	  Valid --status values: pending, ready, dispatched, completed, failed, blocked.
//
// #136은 이 목록을 adapter의 방어적 집합에 맞췄다가 틀렸다. 코드끼리 대조하면
// 두 정의가 같아져도 둘 다 실제와 다를 수 있다(#145, #142에서 실측).
func TestKnownTaskStatusCoversTheOrcaVocabulary(t *testing.T) {
	for _, status := range []string{"pending", "ready", "dispatched", "completed", "failed", "blocked"} {
		t.Run(status, func(t *testing.T) {
			if !knownTaskStatus(status) {
				t.Fatalf("%q is a status Orca can report; classifying it as unknown hides a real inventory row", status)
			}
		})
	}
}

// 종결은 dispatch될 수 없고 worker를 붙들지 않는 상태다. 나머지 넷은 아직
// 실행되거나 실행을 기다리므로 소유자를 잃으면 진짜 잔여물이다.
func TestOnlyFinishedTaskStatusesCountAsSettled(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		t.Run("settled/"+status, func(t *testing.T) {
			if !settledTaskStatus(status) {
				t.Fatalf("%q cannot be dispatched and holds no worker; it must count as settled", status)
			}
		})
	}
	for _, status := range []string{"pending", "ready", "dispatched", "blocked"} {
		t.Run("unsettled/"+status, func(t *testing.T) {
			if settledTaskStatus(status) {
				t.Fatalf("%q can still hold or acquire a worker; treating it as settled loses residue detection", status)
			}
		})
	}
}

// orca가 거부하는 값은 어휘에 없다. 관측될 수 없는 값을 종결로 인정하면 어휘의
// 출처가 흐려지고, 방어도 되지 않는다 — 모르는 상태는 이미 unknown이다.
func TestStatusesOrcaRejectsAreNotInTheVocabulary(t *testing.T) {
	for _, status := range []string{"complete", "cancelled", "canceled", "closed"} {
		t.Run(status, func(t *testing.T) {
			if knownTaskStatus(status) {
				t.Fatalf("Orca rejects %q on task-update; it must not appear in this vocabulary", status)
			}
		})
	}
}

// 어휘 밖의 값은 여전히 unknown이다. 모르는 상태를 조용히 통과시키면 fail-open이 된다.
func TestUnrecognisedTaskStatusStaysUnknown(t *testing.T) {
	for _, status := range []string{"", "running", "paused", "archived"} {
		if knownTaskStatus(status) {
			t.Fatalf("%q is outside the Orca vocabulary and must stay unknown", status)
		}
	}
}
