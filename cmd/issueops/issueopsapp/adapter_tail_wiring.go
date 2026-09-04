package issueopsapp

import (
	installclit4deps "issueops/cmd/issueops/installcli"
	projectclit4deps "issueops/cmd/issueops/projectcli"
	nativeintegrationt4deps "issueops/cmd/issueops/validationcli/nativeintegration"
	agyt4deps "issueops/internal/adapter/agy"
	claudet4deps "issueops/internal/adapter/claude"
	codext4deps "issueops/internal/adapter/codex"
	installadapter "issueops/internal/adapter/install"
	installutiladapter "issueops/internal/adapter/installutil"
	fingerprintt4deps "issueops/internal/adapter/lifecycle/fingerprint"
	omot4deps "issueops/internal/adapter/omo"
	projectbootstrapt4deps "issueops/internal/adapter/projectbootstrap"
	projectdocsadapter "issueops/internal/adapter/projectdocs"
	installcontract "issueops/internal/contract/install"
	mcpdomain "issueops/internal/domain/mcp"
)

// configureAdapterTail은 설치 계획 수립과 프로젝트 문서 관측을 설치한다.
func configureAdapterTail() {
	claudet4deps.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	claudet4deps.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	claudet4deps.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	claudet4deps.HookGroupContainsCommand = installutiladapter.HookGroupContainsCommand
	claudet4deps.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	claudet4deps.HookTargetGenerationMessages = func(config map[string]any, h, expected, running string, read func(string) string) []string {
		return installutiladapter.HookTargetGenerationMessages(config, h, expected, running, read)
	}
	claudet4deps.RunningBuildGenerationString = installadapter.RunningBuildGenerationString
	claudet4deps.FileBuildGenerationString = installadapter.FileBuildGenerationString
	claudet4deps.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	claudet4deps.SemanticSHA256 = installutiladapter.SemanticSHA256
	claudet4deps.ValidateHookConfigForMerge = installutiladapter.ValidateHookConfigForMerge
	claudet4deps.VerifyHookActivation = installutiladapter.VerifyHookActivation
	codext4deps.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	codext4deps.HookGroupContainsAgentHarness = installutiladapter.HookGroupContainsAgentHarness
	codext4deps.HookGroupContainsCommand = installutiladapter.HookGroupContainsCommand
	codext4deps.HookTargetDriftMessages = installutiladapter.HookTargetDriftMessages
	codext4deps.HookTargetGenerationMessages = func(config map[string]any, h, expected, running string, read func(string) string) []string {
		return installutiladapter.HookTargetGenerationMessages(config, h, expected, running, read)
	}
	codext4deps.RunningBuildGenerationString = installadapter.RunningBuildGenerationString
	codext4deps.FileBuildGenerationString = installadapter.FileBuildGenerationString
	codext4deps.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	codext4deps.SemanticSHA256 = installutiladapter.SemanticSHA256
	codext4deps.ValidateHookConfigForMerge = installutiladapter.ValidateHookConfigForMerge
	codext4deps.VerifyHookActivation = installutiladapter.VerifyHookActivation
	omot4deps.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	omot4deps.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	omot4deps.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	omot4deps.SemanticSHA256 = installutiladapter.SemanticSHA256
	omot4deps.MCPCatalogSHA256 = func() (string, error) {
		return installutiladapter.SemanticSHA256(mcpdomain.AdvertisedTools())
	}
	agyt4deps.CaptureNativeActivationEvidence = installutiladapter.CaptureNativeActivationEvidence
	agyt4deps.EnsureSymlinkPlan = installutiladapter.EnsureSymlinkPlan
	agyt4deps.PlanHostSkillLinks = installutiladapter.PlanHostSkillLinks
	agyt4deps.SemanticSHA256 = installutiladapter.SemanticSHA256
	agyt4deps.MCPCatalogSHA256 = func() (string, error) {
		return installutiladapter.SemanticSHA256(mcpdomain.AdvertisedTools())
	}
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
	nativeintegrationt4deps.ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	nativeintegrationt4deps.CodexHooksConfig = codext4deps.HooksConfig
	nativeintegrationt4deps.OmoLifecycleExtension = omot4deps.LifecycleExtension
	nativeintegrationt4deps.VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
	projectbootstrapt4deps.AnalyzeProjectSignals = projectdocsadapter.AnalyzeProjectSignals
	projectbootstrapt4deps.RenderAgentsWithBlock = projectdocsadapter.RenderAgentsWithBlock
	projectbootstrapt4deps.RenderProjectDocs = projectdocsadapter.RenderProjectDocs
	projectclit4deps.AppendProjectDocsEntry = projectdocsadapter.AppendProjectDocsEntry
	projectclit4deps.RouteProjectDocs = projectdocsadapter.RouteProjectDocs
}
