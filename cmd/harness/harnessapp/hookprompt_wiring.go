package harnessapp

import (
	"agent-harness/cmd/harness/hookcli"
	"agent-harness/cmd/harness/hookcli/hookcatalog"
	hookpromptadapter "agent-harness/internal/adapter/hookprompt"
)

// configureHookPrompts는 prompt hint 구성을 설치한다.
func configureHookPrompts() {
	hookcli.BuildUserPromptMCPHints = hookpromptadapter.BuildUserPromptMCPHints
	hookcli.StopOrchestrationRelayFacts = hookpromptadapter.StopOrchestrationRelayFacts
	hookcatalog.BuildProjectDocCatalogContext = hookpromptadapter.BuildProjectDocCatalogContext
}
