package nativeintegration

import (
	codexadapter "issueops/internal/adapter/codex"
	installadapter "issueops/internal/adapter/install"
	installutiladapter "issueops/internal/adapter/installutil"
	omoadapter "issueops/internal/adapter/omo"
)

func init() {
	ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	CodexHooksConfig = codexadapter.HooksConfig
	OmoLifecycleExtension = omoadapter.LifecycleExtension
	VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
