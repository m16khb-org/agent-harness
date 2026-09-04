package validationcli

import (
	nativeintegrationt4d "issueops/cmd/issueops/validationcli/nativeintegration"
	codexadapter "issueops/internal/adapter/codex"
	installadapter "issueops/internal/adapter/install"
	installutiladapter "issueops/internal/adapter/installutil"
	omoadapter "issueops/internal/adapter/omo"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	nativeintegrationt4d.SkillNamesForHost = installutiladapter.SkillNamesForHost
	nativeintegrationt4d.ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	nativeintegrationt4d.CodexHooksConfig = codexadapter.HooksConfig
	nativeintegrationt4d.OmoLifecycleExtension = omoadapter.LifecycleExtension
	nativeintegrationt4d.VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
