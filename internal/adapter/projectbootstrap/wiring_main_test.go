package projectbootstrap

import (
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"os"
	"testing"
)

// 프로덕션에서는 composition root가 주입한다. 테스트는 실제 어댑터를 검증하므로
// 같은 배선을 여기서 재현한다.
func TestMain(m *testing.M) {
	ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
	os.Exit(m.Run())
}
