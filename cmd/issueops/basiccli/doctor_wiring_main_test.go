package basiccli

import (
	"issueops/internal/adapter/doctor"
	lifecycle "issueops/internal/adapter/lifecycle"
	"os"
	"testing"
)

// 프로덕션에서는 issueopsapp이 주입한다. 진단 CLI 테스트는 실제 doctor 동작을
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	doctor.ConfigureLifecycle(lifecycle.ValidateProjectLifecycleState)
	ConfigureDoctor(doctor.HarnessDoctor)
	os.Exit(m.Run())
}
