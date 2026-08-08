package hookcli

import (
	hookcataloghpd "agent-harness/cmd/harness/hookcli/hookcatalog"
	hookpromptadapter "agent-harness/internal/adapter/hookprompt"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	BuildUserPromptMCPHints = hookpromptadapter.BuildUserPromptMCPHints
	StopOrchestrationRelayFacts = hookpromptadapter.StopOrchestrationRelayFacts
	hookcataloghpd.BuildProjectDocCatalogContext = hookpromptadapter.BuildProjectDocCatalogContext
}
