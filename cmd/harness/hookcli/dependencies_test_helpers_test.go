package hookcli

import (
	hookfailurecli "agent-harness/cmd/harness/hookcli/hookfailure"
	hookfailureadapter "agent-harness/internal/adapter/hookfailure"
	"agent-harness/internal/adapter/hookmetrics"
)

// production wiring과 같은 구현을 설치한다. hookcli는 failure 하위 CLI로
// 위임하므로 그쪽 의존도 함께 채운다. fitness graph는 test import를 수집하지
// 않으므로 여기서는 concrete를 써도 된다.
func init() {
	RecordHookMetricEvent = hookmetrics.RecordHookMetricEvent
	PruneHookFailureLog = hookfailureadapter.PruneHookFailureLog
	PruneHookMetricsLog = hookmetrics.PruneHookMetricsLog

	hookfailurecli.RecordHookFailureEvent = hookfailureadapter.RecordHookFailureEvent
	hookfailurecli.ListHookFailureEvents = hookfailureadapter.ListHookFailureEvents
	hookfailurecli.PruneHookFailureLog = hookfailureadapter.PruneHookFailureLog
	hookfailurecli.SummarizeHookFailureLog = hookfailureadapter.SummarizeHookFailureLog
	hookfailurecli.SummarizeHookMetricsLog = hookmetrics.SummarizeHookMetricsLog
	hookfailurecli.MetricsRate = hookmetrics.Rate
}
