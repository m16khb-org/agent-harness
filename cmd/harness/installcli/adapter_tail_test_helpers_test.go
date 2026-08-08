package installcli

import (
	claudet4d "agent-harness/internal/adapter/claude"
	codext4d "agent-harness/internal/adapter/codex"
	installutiladapter "agent-harness/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	claudet4d.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	claudet4d.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	claudet4d.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	claudet4d.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	claudet4d.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	claudet4d.PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
	claudet4d.SemanticSHA256 = installutiladapter.SemanticSHA256
	claudet4d.StopEnforcementFlags = installutiladapter.StopEnforcementFlags
	claudet4d.VerifyHookActivation = installutiladapter.VerifyHookActivation
	codext4d.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	codext4d.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	codext4d.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	codext4d.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	codext4d.PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
	codext4d.SemanticSHA256 = installutiladapter.SemanticSHA256
	codext4d.StopEnforcementFlags = installutiladapter.StopEnforcementFlags
	codext4d.VerifyHookActivation = installutiladapter.VerifyHookActivation
}
