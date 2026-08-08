package validationcli

import (
	stateiotdeps "agent-harness/cmd/harness/selfworkflow/stateio"
	summarytdeps "agent-harness/cmd/harness/selfworkflow/summary"
	webfetchtdeps "agent-harness/cmd/harness/validationcli/webfetch"
	failurecauseadapter "agent-harness/internal/adapter/failurecause"
	webfetchadapter "agent-harness/internal/adapter/outbound/webfetch"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	stateiotdeps.Classify = failurecauseadapter.Classify
	summarytdeps.Classify = failurecauseadapter.Classify
	webfetchtdeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchtdeps.RunBenchmark = webfetchadapter.RunBenchmark
}
