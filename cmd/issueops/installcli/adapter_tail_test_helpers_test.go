package installcli

import (
	claudet4d "issueops/internal/adapter/claude"
	codext4d "issueops/internal/adapter/codex"
	installutiladapter "issueops/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	claudet4d.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	claudet4d.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	claudet4d.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	claudet4d.HookGroupContainsCommand = installutiladapter.HookGroupContainsCommand
	claudet4d.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	claudet4d.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	claudet4d.SemanticSHA256 = installutiladapter.SemanticSHA256
	claudet4d.VerifyHookActivation = installutiladapter.VerifyHookActivation
	codext4d.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	codext4d.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	codext4d.HookGroupContainsCommand = installutiladapter.HookGroupContainsCommand
	codext4d.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	codext4d.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	codext4d.SemanticSHA256 = installutiladapter.SemanticSHA256
	codext4d.VerifyHookActivation = installutiladapter.VerifyHookActivation
}
