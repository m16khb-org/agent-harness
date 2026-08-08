package hookfailure

import (
	hookfailureadapter "agent-harness/internal/adapter/hookfailure"
	"agent-harness/internal/adapter/hookmetrics"
)

// production wiring과 같은 로그 구현을 설치한다. fitness graph는 test import를
// 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	RecordHookFailureEvent = hookfailureadapter.RecordHookFailureEvent
	ListHookFailureEvents = hookfailureadapter.ListHookFailureEvents
	PruneHookFailureLog = hookfailureadapter.PruneHookFailureLog
	SummarizeHookFailureLog = hookfailureadapter.SummarizeHookFailureLog
	SummarizeHookMetricsLog = hookmetrics.SummarizeHookMetricsLog
	MetricsRate = hookmetrics.Rate
}
