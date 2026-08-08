package hookcli

import (
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	nextaction "agent-harness/internal/domain/nextaction"
)

// lifecycle 훅 연산은 사용자 상태를 읽고 쓰는 I/O다. hookcli는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다. 기본값은 훅을 실패시키지 않는
// 중립 응답이다.
var (
	recordLifecycleToolUse = func(req lifecyclecontract.HookToolUseLifecycleRequest) (lifecyclecontract.HookToolUseLifecycleResult, error) {
		return lifecyclecontract.HookToolUseLifecycleResult{}, nil
	}
	sourceCheckoutMisdirectWarning  = func(req lifecyclecontract.HookToolUseLifecycleRequest) (string, string) { return "", "" }
	buildLifecyclePreCompactCapsule = func(repo string) lifecyclecontract.LifecycleCompactResult {
		return lifecyclecontract.LifecycleCompactResult{}
	}
	buildLifecycleStopReminder = func(repo string) lifecyclecontract.LifecycleStopReminderResult {
		return lifecyclecontract.LifecycleStopReminderResult{}
	}
	buildLifecyclePreToolUseDecision = func(req lifecyclecontract.HookToolUseLifecycleRequest) lifecyclecontract.HookPreToolUseDecisionResult {
		return lifecyclecontract.HookPreToolUseDecisionResult{}
	}
	recordStopNextActionRelay = func(repo string, trigger nextaction.NextActionJudgementTriggerResult) lifecyclecontract.StopNextActionRelayResult {
		return lifecyclecontract.StopNextActionRelayResult{}
	}
	clearStopNextActionRelay = func(repo string) lifecyclecontract.StopNextActionRelayResult {
		return lifecyclecontract.StopNextActionRelayResult{}
	}
	buildLifecyclePostCompactReminder = func(repo string) lifecyclecontract.LifecycleCompactResult {
		return lifecyclecontract.LifecycleCompactResult{}
	}
)

// LifecycleDeps는 composition root가 실제 lifecycle 어댑터를 꽂는 진입점이다.
type LifecycleDeps struct {
	RecordLifecycleToolUse            func(lifecyclecontract.HookToolUseLifecycleRequest) (lifecyclecontract.HookToolUseLifecycleResult, error)
	SourceCheckoutMisdirectWarning    func(lifecyclecontract.HookToolUseLifecycleRequest) (string, string)
	BuildLifecyclePreCompactCapsule   func(string) lifecyclecontract.LifecycleCompactResult
	BuildLifecycleStopReminder        func(string) lifecyclecontract.LifecycleStopReminderResult
	BuildLifecyclePreToolUseDecision  func(lifecyclecontract.HookToolUseLifecycleRequest) lifecyclecontract.HookPreToolUseDecisionResult
	RecordStopNextActionRelay         func(string, nextaction.NextActionJudgementTriggerResult) lifecyclecontract.StopNextActionRelayResult
	ClearStopNextActionRelay          func(string) lifecyclecontract.StopNextActionRelayResult
	BuildLifecyclePostCompactReminder func(string) lifecyclecontract.LifecycleCompactResult
}

func ConfigureLifecycle(deps LifecycleDeps) {
	if deps.RecordLifecycleToolUse != nil {
		recordLifecycleToolUse = deps.RecordLifecycleToolUse
	}
	if deps.SourceCheckoutMisdirectWarning != nil {
		sourceCheckoutMisdirectWarning = deps.SourceCheckoutMisdirectWarning
	}
	if deps.BuildLifecyclePreCompactCapsule != nil {
		buildLifecyclePreCompactCapsule = deps.BuildLifecyclePreCompactCapsule
	}
	if deps.BuildLifecycleStopReminder != nil {
		buildLifecycleStopReminder = deps.BuildLifecycleStopReminder
	}
	if deps.BuildLifecyclePreToolUseDecision != nil {
		buildLifecyclePreToolUseDecision = deps.BuildLifecyclePreToolUseDecision
	}
	if deps.RecordStopNextActionRelay != nil {
		recordStopNextActionRelay = deps.RecordStopNextActionRelay
	}
	if deps.ClearStopNextActionRelay != nil {
		clearStopNextActionRelay = deps.ClearStopNextActionRelay
	}
	if deps.BuildLifecyclePostCompactReminder != nil {
		buildLifecyclePostCompactReminder = deps.BuildLifecyclePostCompactReminder
	}
}
