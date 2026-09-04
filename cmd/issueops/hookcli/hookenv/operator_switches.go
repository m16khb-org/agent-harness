package hookenv

import "os"

// OperatorSwitches는 운영자가 셸에 켜 두는 harness 환경 스위치다. 프로덕션에서는
// 정상 기능이지만 테스트 진입점에서는 오염원이다 — 상속된 값이 hook 동작을 바꿔
// 버려서, 같은 코드가 CI(변수 없음)와 dogfood 셸(변수 있음)에서 다른 결론을 낸다.
var OperatorSwitches = []string{
	"ISSUEOPS_DISABLE_HOOKS",
	"ISSUEOPS_SELF_VERIFY_LLM_EVAL",
}

// ClearInheritedOperatorSwitches는 상속된 운영자 환경 스위치를 제거하고, 실제로
// 제거된 변수의 이름을 돌려준다.
//
// 개별 테스트가 아니라 패키지 진입점(TestMain)에서 한 번 부르는 것이 계약이다.
// 개별 테스트에 맡기면 새로 추가되는 테스트가 같은 함정에 반복해서 빠진다 —
// 실제로 #395는 kill-switch를 **켜는** 테스트만 t.Setenv로 격리하고 그것을
// 전제로 하는 대조군들은 격리하지 않아 발생했다.
//
// 스위치를 켜고 검증하는 테스트는 그대로 t.Setenv를 쓰면 된다. 이 함수는 시작
// 시점의 상속값만 지우므로 그 의도를 침범하지 않는다.
func ClearInheritedOperatorSwitches() []string {
	cleared := []string{}
	for _, name := range OperatorSwitches {
		if _, present := os.LookupEnv(name); !present {
			continue
		}
		if err := os.Unsetenv(name); err == nil {
			cleared = append(cleared, name)
		}
	}
	return cleared
}
