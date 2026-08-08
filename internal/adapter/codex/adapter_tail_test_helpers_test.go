package codex

import (
	installadapter "agent-harness/internal/adapter/install"
	installutiladapter "agent-harness/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	RunningBuildGenerationString = installadapter.RunningBuildGenerationString
	FileBuildGenerationString = installadapter.FileBuildGenerationString
	HookTargetGenerationMessages = func(config map[string]any, h, expected, running string, read func(string) string) []string {
		return installutiladapter.HookTargetGenerationMessages(config, h, expected, running, read)
	}
	PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
	SemanticSHA256 = installutiladapter.SemanticSHA256
	StopEnforcementFlags = installutiladapter.StopEnforcementFlags
	VerifyHookActivation = installutiladapter.VerifyHookActivation
}
