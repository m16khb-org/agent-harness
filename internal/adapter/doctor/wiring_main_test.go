package doctor

import (
	lifecycle "issueops/internal/adapter/lifecycle"
	"os"
	"testing"
)

// 프로덕션에서는 composition root가 주입한다. doctor 테스트는 실제 lifecycle
// 상태 검증을 전제로 하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	ConfigureLifecycle(lifecycle.ValidateProjectLifecycleState)
	os.Exit(m.Run())
}
