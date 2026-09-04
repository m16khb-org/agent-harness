package validationcli

import (
	stateiotdeps "issueops/cmd/issueops/selfworkflow/stateio"
	summarytdeps "issueops/cmd/issueops/selfworkflow/summary"
	webfetchtdeps "issueops/cmd/issueops/validationcli/webfetch"
	failurecauseadapter "issueops/internal/adapter/failurecause"
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	stateiotdeps.Classify = failurecauseadapter.Classify
	summarytdeps.Classify = failurecauseadapter.Classify
	webfetchtdeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchtdeps.RunBenchmark = webfetchadapter.RunBenchmark
}
