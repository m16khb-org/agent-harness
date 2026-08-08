package adapter_test

import (
	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	installutiladapter "agent-harness/internal/adapter/installutil"
)

// production wiring과 같은 설치 유틸을 설치한다. 이 테스트는 두 host adapter의
// 계약을 함께 확인하므로 양쪽 의존을 모두 채운다.
func init() {
	for _, set := range []func(){
		func() {
			claudeadapter.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
			claudeadapter.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
			claudeadapter.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
			claudeadapter.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
			claudeadapter.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
			claudeadapter.PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
			claudeadapter.SemanticSHA256 = installutiladapter.SemanticSHA256
			claudeadapter.StopEnforcementFlags = installutiladapter.StopEnforcementFlags
			claudeadapter.VerifyHookActivation = installutiladapter.VerifyHookActivation
		},
		func() {
			codexadapter.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
			codexadapter.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
			codexadapter.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
			codexadapter.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
			codexadapter.PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
			codexadapter.SemanticSHA256 = installutiladapter.SemanticSHA256
			codexadapter.StopEnforcementFlags = installutiladapter.StopEnforcementFlags
			codexadapter.VerifyHookActivation = installutiladapter.VerifyHookActivation
		},
	} {
		set()
	}
}
