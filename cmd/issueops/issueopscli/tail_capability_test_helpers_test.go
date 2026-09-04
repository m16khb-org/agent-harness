package issueopscli

import (
	remotecmdtdeps "issueops/cmd/issueops/issueopscli/remotecmd"
	mcpclitdeps "issueops/cmd/issueops/mcpcli"
	stateiotdeps "issueops/cmd/issueops/selfworkflow/stateio"
	summarytdeps "issueops/cmd/issueops/selfworkflow/summary"
	auditadapter "issueops/internal/adapter/audit"
	failurecauseadapter "issueops/internal/adapter/failurecause"
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
	provideradapter "issueops/internal/adapter/provider"
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
