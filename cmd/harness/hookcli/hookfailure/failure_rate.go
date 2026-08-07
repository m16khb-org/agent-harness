package hookfailure

import (
	hookfailureadapter "agent-harness/internal/adapter/hookfailure"
	"agent-harness/internal/adapter/hookmetrics"
)

// HookFailureRateStats는 실패 로그와 호출 횟수를 결합한 결과다.
type HookFailureRateStats struct {
	hookfailureadapter.HookFailureStats
	Invocations        map[string]int     `json:"invocations,omitempty"`
	FailureRate        map[string]float64 `json:"failure_rate,omitempty"`
	FailureRateOverall float64            `json:"failure_rate_overall"`
}

// SummarizeHookFailureStats는 실패 로그와 metrics의 호출 횟수를 합쳐 hook별
// failure_rate = failures/invocations와 전체 비율을 계산한다.
//
// 두 로그를 결합하는 것이 이 함수의 전부이고 어느 한쪽의 관심사도 아니므로,
// 두 로그를 함께 읽는 유일한 소비자인 이 CLI 패키지가 소유한다. metrics 조회가
// 실패해도 실패 통계 자체는 그대로 반환한다 — 비율만 비어 있게 된다.
func SummarizeHookFailureStats() (HookFailureRateStats, error) {
	fstats, err := hookfailureadapter.SummarizeHookFailureLog()
	out := HookFailureRateStats{
		HookFailureStats: fstats,
		Invocations:      map[string]int{},
		FailureRate:      map[string]float64{},
	}
	if err != nil {
		out.OK = false
		return out, err
	}
	mstats, mErr := hookmetrics.SummarizeHookMetricsLog()
	if mErr == nil {
		for hook, lat := range mstats.ByHook {
			out.Invocations[hook] = lat.Count
		}
	}
	for hook, failures := range fstats.ByHook {
		if inv := out.Invocations[hook]; inv > 0 {
			out.FailureRate[hook] = hookmetrics.Rate(failures, inv)
		}
	}
	out.FailureRateOverall = hookmetrics.Rate(fstats.Total, mstats.Total)
	return out, nil
}
