package hookcatalog

import (
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. 이 패키지 테스트는 실제 lifecycle
// 상태를 전제로 하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	projectbootstrap.ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
	os.Exit(m.Run())
}
