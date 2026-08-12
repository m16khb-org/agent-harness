package nativeintegration

import (
	codexadapter "agent-harness/internal/adapter/codex"
	installadapter "agent-harness/internal/adapter/install"
	installutiladapter "agent-harness/internal/adapter/installutil"
	omoadapter "agent-harness/internal/adapter/omo"
)

func init() {
	ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	CodexHooksConfig = codexadapter.HooksConfig
	OmoLifecycleExtension = omoadapter.LifecycleExtension
	VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
