package hookfailure

import (
	"time"

	hookfailurecontract "agent-harness/internal/contract/hookfailure"
	hookmetricscontract "agent-harness/internal/contract/hookmetrics"
)

// hook failure/metric 로그 연산은 composition root가 설치한다. CLI는 flag
// 해석과 출력만 소유하고 로그를 어디에 어떻게 쌓는지 알지 않는다.
var (
	RecordHookFailureEvent  func(hookfailurecontract.HookFailureEvent) (hookfailurecontract.HookFailureRecordResult, error)
	ListHookFailureEvents   func(limit int) (hookfailurecontract.HookFailureListResult, error)
	PruneHookFailureLog     func(maxAge time.Duration) (hookfailurecontract.HookFailurePruneResult, error)
	SummarizeHookFailureLog func() (hookfailurecontract.HookFailureStats, error)
	SummarizeHookMetricsLog func() (hookmetricscontract.HookMetricsStats, error)
	PruneHookMetricsLog     func(maxAge time.Duration) (hookmetricscontract.HookMetricsPruneResult, error)
	MetricsRate             func(num, denom int) float64
)
