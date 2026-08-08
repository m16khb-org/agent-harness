package hookenv

import (
	"os"
	"slices"
	"testing"
)

// TestClearInheritedOperatorSwitchesRemovesEverySwitch는 격리 계약 자체를
// 고정한다. 이 테스트가 없으면 목록에 새 스위치를 추가하고도 격리를 잊는다.
func TestClearInheritedOperatorSwitchesRemovesEverySwitch(t *testing.T) {
	for _, name := range OperatorSwitches {
		t.Setenv(name, "1")
	}

	cleared := ClearInheritedOperatorSwitches()

	for _, name := range OperatorSwitches {
		if value, present := os.LookupEnv(name); present {
			t.Fatalf("%s가 지워지지 않았다: %q", name, value)
		}
		if !slices.Contains(cleared, name) {
			t.Fatalf("%s가 제거 보고에 없다: %v", name, cleared)
		}
	}
}

// TestClearInheritedOperatorSwitchesIsIdempotent는 변수가 애초에 없을 때
// (CI 환경) 아무것도 보고하지 않음을 고정한다.
func TestClearInheritedOperatorSwitchesIsIdempotent(t *testing.T) {
	for _, name := range OperatorSwitches {
		t.Setenv(name, "1")
	}
	ClearInheritedOperatorSwitches()

	if cleared := ClearInheritedOperatorSwitches(); len(cleared) != 0 {
		t.Fatalf("이미 비어 있는 환경에서 제거를 보고하면 안 된다: %v", cleared)
	}
}

// TestEnforcementSwitchReadsFalseAfterIsolation은 #395의 실패 조건을 직접
// 재현한다. 상속된 kill-switch가 켜져 있어도 격리 후에는 enforcement 판독이
// 꺼짐으로 돌아와야 한다 — 이것이 dogfood 셸과 CI가 같은 결론을 내는 근거다.
func TestEnforcementSwitchReadsFalseAfterIsolation(t *testing.T) {
	t.Setenv("HARNESS_DISABLE_HOOKS", "1")
	if !Bool("HARNESS_DISABLE_HOOKS") {
		t.Fatal("전제 확인 실패: 켜 둔 스위치는 true로 읽혀야 한다")
	}

	ClearInheritedOperatorSwitches()

	if Bool("HARNESS_DISABLE_HOOKS") {
		t.Fatal("격리 후에는 스위치가 꺼진 것으로 읽혀야 한다")
	}
}
