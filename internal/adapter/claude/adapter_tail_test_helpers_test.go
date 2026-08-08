package claude

import (
	installutiladapter "agent-harness/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
	SemanticSHA256 = installutiladapter.SemanticSHA256
	StopEnforcementFlags = installutiladapter.StopEnforcementFlags
	VerifyHookActivation = installutiladapter.VerifyHookActivation
}
