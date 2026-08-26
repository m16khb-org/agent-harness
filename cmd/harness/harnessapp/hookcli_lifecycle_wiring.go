package harnessapp

import (
	"agent-harness/cmd/harness/hookcli"
	lifecycle "agent-harness/internal/adapter/lifecycle"
)

// hook 명령은 lifecycle 어댑터를 직접 알지 않는다. 실제 구현을 아는 곳은
// composition root 하나뿐이다.
func configureHookCLILifecycle() {
	hookcli.ConfigureLifecycle(hookcli.LifecycleDeps{
		RecordLifecycleToolUse:           lifecycle.RecordLifecycleToolUse,
		SourceCheckoutMisdirectWarning:   lifecycle.SourceCheckoutMisdirectWarning,
		BuildLifecyclePreCompactCapsule:  lifecycle.BuildLifecyclePreCompactCapsule,
		BuildLifecycleStopReminder:       lifecycle.BuildLifecycleStopReminder,
		BuildLifecyclePreToolUseDecision: lifecycle.BuildLifecyclePreToolUseDecision,
		RecordStopNextActionRelay:        lifecycle.RecordStopNextActionRelay,
		ClearStopNextActionRelay:         lifecycle.ClearStopNextActionRelay,
	})
}
