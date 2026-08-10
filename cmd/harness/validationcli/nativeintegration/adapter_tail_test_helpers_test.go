package nativeintegration

import (
	codexadapter "agent-harness/internal/adapter/codex"
	installadapter "agent-harness/internal/adapter/install"
	installutiladapter "agent-harness/internal/adapter/installutil"
)

func init() {
	ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	CodexHooksConfig = codexadapter.HooksConfig
	VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
