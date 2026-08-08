package hookcli

import (
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. 훅 계약 테스트는 실제 lifecycle 동작을
// 검증하므로 같은 배선을 여기서 재현한다.
func TestMain(m *testing.M) {
	ConfigureLifecycle(LifecycleDeps{
		RecordLifecycleToolUse:            lifecycle.RecordLifecycleToolUse,
		SourceCheckoutMisdirectWarning:    lifecycle.SourceCheckoutMisdirectWarning,
		BuildLifecyclePreCompactCapsule:   lifecycle.BuildLifecyclePreCompactCapsule,
		BuildLifecycleStopReminder:        lifecycle.BuildLifecycleStopReminder,
		BuildLifecyclePreToolUseDecision:  lifecycle.BuildLifecyclePreToolUseDecision,
		RecordStopNextActionRelay:         lifecycle.RecordStopNextActionRelay,
		ClearStopNextActionRelay:          lifecycle.ClearStopNextActionRelay,
		BuildLifecyclePostCompactReminder: lifecycle.BuildLifecyclePostCompactReminder,
	})
	projectbootstrap.ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
	os.Exit(m.Run())
}
