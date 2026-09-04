package claude

import (
	installadapter "issueops/internal/adapter/install"
	installutiladapter "issueops/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	HookGroupContainsCommand = installutiladapter.HookGroupContainsCommand
	HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	RunningBuildGenerationString = installadapter.RunningBuildGenerationString
	FileBuildGenerationString = installadapter.FileBuildGenerationString
	HookTargetGenerationMessages = func(config map[string]any, h, expected, running string, read func(string) string) []string {
		return installutiladapter.HookTargetGenerationMessages(config, h, expected, running, read)
	}
	PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	SemanticSHA256 = installutiladapter.SemanticSHA256
	ValidateHookConfigForMerge = installutiladapter.ValidateHookConfigForMerge
	VerifyHookActivation = installutiladapter.VerifyHookActivation
}
