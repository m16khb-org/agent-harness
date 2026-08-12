package adapter_test

import (
	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	installutiladapter "agent-harness/internal/adapter/installutil"
	omoadapter "agent-harness/internal/adapter/omo"
)

// production wiring과 같은 설치 유틸을 설치한다. 이 테스트는 세 host adapter의
// 계약을 함께 확인하므로 모든 의존을 채운다.
func init() {
	claudeadapter.NewInstallPlan = func(host string, dryRun bool) claudeadapter.InstallPlan {
		return installutiladapter.NewPlan(host, dryRun)
	}
	claudeadapter.WriteJSONPlan = installutiladapter.WriteJSONPlan
	claudeadapter.WriteTextPlan = installutiladapter.WriteTextPlan
	claudeadapter.TOMLString = installutiladapter.TOMLString
	codexadapter.NewInstallPlan = func(host string, dryRun bool) codexadapter.InstallPlan {
		return installutiladapter.NewPlan(host, dryRun)
	}
	codexadapter.WriteJSONPlan = installutiladapter.WriteJSONPlan
	codexadapter.WriteTextPlan = installutiladapter.WriteTextPlan
	codexadapter.TOMLString = installutiladapter.TOMLString
	omoadapter.NewInstallPlan = func(host string, dryRun bool) omoadapter.InstallPlan {
		return installutiladapter.NewPlan(host, dryRun)
	}
	omoadapter.WriteJSONPlan = installutiladapter.WriteJSONPlan
	omoadapter.WriteTextPlan = installutiladapter.WriteTextPlan
	for _, set := range []func(){
		func() {
			claudeadapter.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
			claudeadapter.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
			claudeadapter.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
			claudeadapter.HookGroupContainsCommand = installutiladapter.HookGroupContainsCommand
			claudeadapter.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
			claudeadapter.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
			claudeadapter.SemanticSHA256 = installutiladapter.SemanticSHA256
			claudeadapter.ValidateHookConfigForMerge = installutiladapter.ValidateHookConfigForMerge
			claudeadapter.VerifyHookActivation = installutiladapter.VerifyHookActivation
		},
		func() {
			codexadapter.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
			codexadapter.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
			codexadapter.HookGroupContainsCommand = installutiladapter.HookGroupContainsCommand
			codexadapter.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
			codexadapter.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
			codexadapter.SemanticSHA256 = installutiladapter.SemanticSHA256
			codexadapter.ValidateHookConfigForMerge = installutiladapter.ValidateHookConfigForMerge
			codexadapter.VerifyHookActivation = installutiladapter.VerifyHookActivation
		},
		func() {
			omoadapter.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
			omoadapter.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
			omoadapter.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
			omoadapter.SemanticSHA256 = installutiladapter.SemanticSHA256
		},
	} {
		set()
	}
}
