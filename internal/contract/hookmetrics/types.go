// Package hookmetrics는 hookmetrics capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package hookmetrics

type HookMetricEvent struct {
	Timestamp  string `json:"timestamp,omitempty"`
	Hook       string `json:"hook"`
	Host       string `json:"host,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	// Decision is set by enforcement gates when they block ("block"); empty
	// for ordinary completions.
	Decision string `json:"decision,omitempty"`
}

type HookMetricRecordResult struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}

type HookLatencyStats struct {
	Count  int   `json:"count"`
	P50MS  int64 `json:"p50_ms"`
	P95MS  int64 `json:"p95_ms"`
	MaxMS  int64 `json:"max_ms"`
	Blocks int   `json:"blocks"`
	// Asks counts enforcement "ask" decisions; like Blocks they are real gate
	// interventions, so both feed GateHitRate (A2/G4).
	Asks int `json:"asks"`
	// GateHitRate = (Blocks+Asks)/Count: the fraction of invocations where the
	// gate actually intervened. A gate that is silently disabled keeps Count
	// rising while this drops to ~0, which absolute Blocks alone cannot reveal.
	GateHitRate float64 `json:"gate_hit_rate"`
}

type HookMetricsStats struct {
	OK     bool                        `json:"ok"`
	Path   string                      `json:"path"`
	Total  int                         `json:"total"`
	ByHook map[string]HookLatencyStats `json:"by_hook,omitempty"`
	// GateHitRate is the overall (Blocks+Asks)/Count across all hooks.
	GateHitRate float64 `json:"gate_hit_rate"`
	Last24h     int     `json:"last_24h"`
}

type HookMetricsPruneResult struct {
	OK               bool   `json:"ok"`
	Path             string `json:"path"`
	Pruned           int    `json:"pruned"`
	Kept             int    `json:"kept"`
	StaleTempRemoved int    `json:"stale_temp_removed,omitempty"`
}
