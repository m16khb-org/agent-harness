package webfetchcli

import (
	webfetchadapter "agent-harness/internal/adapter/outbound/webfetch"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	DeterministicFixtures = webfetchadapter.DeterministicFixtures
	Fetch = webfetchadapter.Fetch
	RunBenchmark = webfetchadapter.RunBenchmark
}
