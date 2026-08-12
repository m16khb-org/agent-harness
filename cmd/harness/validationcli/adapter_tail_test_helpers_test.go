package validationcli

import (
	nativeintegrationt4d "agent-harness/cmd/harness/validationcli/nativeintegration"
	codexadapter "agent-harness/internal/adapter/codex"
	installadapter "agent-harness/internal/adapter/install"
	installutiladapter "agent-harness/internal/adapter/installutil"
	omoadapter "agent-harness/internal/adapter/omo"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	nativeintegrationt4d.SkillNamesForHost = installutiladapter.SkillNamesForHost
	nativeintegrationt4d.ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	nativeintegrationt4d.CodexHooksConfig = codexadapter.HooksConfig
	nativeintegrationt4d.OmoLifecycleExtension = omoadapter.LifecycleExtension
	nativeintegrationt4d.VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
