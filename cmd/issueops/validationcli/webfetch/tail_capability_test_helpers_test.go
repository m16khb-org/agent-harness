package webfetch

import (
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	DeterministicFixtures = webfetchadapter.DeterministicFixtures
	RunBenchmark = webfetchadapter.RunBenchmark
}
