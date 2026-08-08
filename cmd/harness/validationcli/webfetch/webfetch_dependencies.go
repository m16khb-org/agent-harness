package webfetch

import (
	webfetchcontract "agent-harness/internal/contract/webfetch"
	"context"
)

// 이 연산들은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	DeterministicFixtures func() []webfetchcontract.BenchmarkFixture
	RunBenchmark          func(ctx context.Context, req webfetchcontract.BenchmarkRequest) (webfetchcontract.BenchmarkResult, error)
)
