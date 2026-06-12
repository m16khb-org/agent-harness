package hookmetrics

import (
	"testing"
	"time"
)

// Q2 phase 2: hooks sit on the per-tool-call critical path with a 5s budget,
// but no latency or gate-hit signal existed. Record + Summarize provide both.
func TestSummarizeHookMetricsAggregatesLatencyAndDecisions(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	for _, e := range []HookMetricEvent{
		{Hook: "pre-tool-use", DurationMS: 10},
		{Hook: "pre-tool-use", DurationMS: 20},
		{Hook: "pre-tool-use", DurationMS: 200, Decision: "block"},
		{Hook: "stop", DurationMS: 50},
	} {
		if _, err := RecordHookMetricEvent(e); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := SummarizeHookMetricsLog()
	if err != nil || !stats.OK {
		t.Fatalf("summarize failed: %+v err=%v", stats, err)
	}
	if stats.Total != 4 {
		t.Fatalf("expected 4 events, got %+v", stats)
	}
	pre := stats.ByHook["pre-tool-use"]
	if pre.Count != 3 || pre.P50MS != 20 || pre.P95MS != 200 || pre.MaxMS != 200 {
		t.Fatalf("unexpected pre-tool-use latency stats: %+v", pre)
	}
	if pre.Blocks != 1 {
		t.Fatalf("expected 1 block for pre-tool-use, got %+v", pre)
	}
	if stop := stats.ByHook["stop"]; stop.Count != 1 || stop.Blocks != 0 {
		t.Fatalf("unexpected stop stats: %+v", stop)
	}
	if stats.Last24h != 4 {
		t.Fatalf("recency window wrong: %+v", stats)
	}
}

func TestSummarizeHookMetricsHandlesMissingLog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stats, err := SummarizeHookMetricsLog()
	if err != nil || !stats.OK || stats.Total != 0 {
		t.Fatalf("missing log must summarize to zero: %+v err=%v", stats, err)
	}
}

func TestPruneHookMetricsLogDropsOldEntries(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := RecordHookMetricEvent(HookMetricEvent{Hook: "stop", DurationMS: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordHookMetricEvent(HookMetricEvent{
		Timestamp: time.Now().UTC().Add(-31 * 24 * time.Hour).Format(time.RFC3339Nano),
		Hook:      "stop", DurationMS: 5,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := PruneHookMetricsLog(720 * time.Hour)
	if err != nil || !result.OK || result.Pruned != 1 || result.Kept != 1 {
		t.Fatalf("unexpected prune result: %+v err=%v", result, err)
	}
}
