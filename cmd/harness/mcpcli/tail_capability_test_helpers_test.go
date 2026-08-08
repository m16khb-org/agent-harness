package mcpcli

import (
	stateiotdeps "agent-harness/cmd/harness/selfworkflow/stateio"
	summarytdeps "agent-harness/cmd/harness/selfworkflow/summary"
	auditadapter "agent-harness/internal/adapter/audit"
	failurecauseadapter "agent-harness/internal/adapter/failurecause"
	webfetchadapter "agent-harness/internal/adapter/outbound/webfetch"
	tracetdeps "agent-harness/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	AuditCommandPolicy = auditadapter.AuditCommandPolicy
	Fetch = webfetchadapter.Fetch
	stateiotdeps.Classify = failurecauseadapter.Classify
	summarytdeps.Classify = failurecauseadapter.Classify
	tracetdeps.Classify = failurecauseadapter.Classify
}
