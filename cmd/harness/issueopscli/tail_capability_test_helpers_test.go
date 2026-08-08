package issueopscli

import (
	remotecmdtdeps "agent-harness/cmd/harness/issueopscli/remotecmd"
	mcpclitdeps "agent-harness/cmd/harness/mcpcli"
	stateiotdeps "agent-harness/cmd/harness/selfworkflow/stateio"
	summarytdeps "agent-harness/cmd/harness/selfworkflow/summary"
	auditadapter "agent-harness/internal/adapter/audit"
	failurecauseadapter "agent-harness/internal/adapter/failurecause"
	webfetchadapter "agent-harness/internal/adapter/outbound/webfetch"
	provideradapter "agent-harness/internal/adapter/provider"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	Resolve = provideradapter.Resolve
	mcpclitdeps.AuditCommandPolicy = auditadapter.AuditCommandPolicy
	mcpclitdeps.Fetch = webfetchadapter.Fetch
	remotecmdtdeps.Resolve = provideradapter.Resolve
	stateiotdeps.Classify = failurecauseadapter.Classify
	summarytdeps.Classify = failurecauseadapter.Classify
}
