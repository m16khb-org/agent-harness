package vcsissue

import (
	"agent-harness/cmd/harness/hookcli"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. pre-tool-use 훅 계약을 실제 lifecycle
// 판정으로 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	hookcli.ConfigureLifecycle(hookcli.LifecycleDeps{
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
