package harnessapp

import (
	installclit4deps "agent-harness/cmd/harness/installcli"
	projectclit4deps "agent-harness/cmd/harness/projectcli"
	nativeintegrationt4deps "agent-harness/cmd/harness/validationcli/nativeintegration"
	claudet4deps "agent-harness/internal/adapter/claude"
	codext4deps "agent-harness/internal/adapter/codex"
	installadapter "agent-harness/internal/adapter/install"
	installutiladapter "agent-harness/internal/adapter/installutil"
	fingerprintt4deps "agent-harness/internal/adapter/lifecycle/fingerprint"
	projectbootstrapt4deps "agent-harness/internal/adapter/projectbootstrap"
	projectdocsadapter "agent-harness/internal/adapter/projectdocs"
	installcontract "agent-harness/internal/contract/install"
)

// configureAdapterTail은 설치 계획 수립과 프로젝트 문서 관측을 설치한다.
func configureAdapterTail() {
	claudet4deps.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	claudet4deps.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	claudet4deps.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	claudet4deps.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	claudet4deps.HookTargetGenerationMessages = func(config map[string]any, h, expected, running string, read func(string) string) []string {
		return installutiladapter.HookTargetGenerationMessages(config, h, expected, running, read)
	}
	claudet4deps.RunningBuildGenerationString = installadapter.RunningBuildGenerationString
	claudet4deps.FileBuildGenerationString = installadapter.FileBuildGenerationString
	claudet4deps.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	claudet4deps.PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
	claudet4deps.SemanticSHA256 = installutiladapter.SemanticSHA256
	claudet4deps.StopEnforcementFlags = installutiladapter.StopEnforcementFlags
	claudet4deps.VerifyHookActivation = installutiladapter.VerifyHookActivation
	codext4deps.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	codext4deps.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	codext4deps.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	codext4deps.HookTargetGenerationMessages = func(config map[string]any, h, expected, running string, read func(string) string) []string {
		return installutiladapter.HookTargetGenerationMessages(config, h, expected, running, read)
	}
	codext4deps.RunningBuildGenerationString = installadapter.RunningBuildGenerationString
	codext4deps.FileBuildGenerationString = installadapter.FileBuildGenerationString
	codext4deps.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	codext4deps.PreToolUseEnforcementFlags = installutiladapter.PreToolUseEnforcementFlags
	codext4deps.SemanticSHA256 = installutiladapter.SemanticSHA256
	codext4deps.StopEnforcementFlags = installutiladapter.StopEnforcementFlags
	codext4deps.VerifyHookActivation = installutiladapter.VerifyHookActivation
	fingerprintt4deps.ReadGitOriginURL = projectdocsadapter.ReadGitOriginURL
	installclit4deps.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	// 관리 대상 명령 파일 채택은 파일시스템 트랜잭션이다. 구현을 아는 곳은
	// composition root 하나뿐이다.
	installclit4deps.PrepareManagedCommandPathCandidate = func(target, candidate, path string, adopt, dryRun bool) (installclit4deps.ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error) {
		transaction, plan, err := installutiladapter.PrepareManagedCommandPathCandidate(target, candidate, path, adopt, dryRun)
		if transaction == nil {
			return nil, plan, err
		}
		return transaction, plan, err
	}
	installclit4deps.SemanticSHA256 = installutiladapter.SemanticSHA256
	nativeintegrationt4deps.SkillNamesForHost = installutiladapter.SkillNamesForHost
	projectbootstrapt4deps.AnalyzeProjectSignals = projectdocsadapter.AnalyzeProjectSignals
	projectbootstrapt4deps.RenderAgentsWithBlock = projectdocsadapter.RenderAgentsWithBlock
	projectbootstrapt4deps.RenderProjectDocs = projectdocsadapter.RenderProjectDocs
	projectclit4deps.AppendProjectDocsRecord = projectdocsadapter.AppendProjectDocsRecord
	projectclit4deps.RouteProjectDocs = projectdocsadapter.RouteProjectDocs
}
