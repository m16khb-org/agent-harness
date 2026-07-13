package hookmetrics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	corestate "agent-harness/internal/core/state"
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

// A2/G4: gate_hit_rate = (blocks+asks)/invocations turns the existing Count
// denominator + Blocks numerator into an enforcement-effectiveness signal so a
// silently-disabled gate (invocations continue, interventions drop to ~0) is
// visible. "ask" is a real enforcement decision and must count toward the rate.
func TestSummarizeHookMetricsGateHitRateCountsBlockAndAsk(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, e := range []HookMetricEvent{
		{Hook: "pre-tool-use", DurationMS: 10},                    // pass
		{Hook: "pre-tool-use", DurationMS: 20, Decision: "block"}, // block
		{Hook: "pre-tool-use", DurationMS: 30, Decision: "ask"},   // ask (was uncounted)
		{Hook: "pre-tool-use", DurationMS: 40},                    // pass
		{Hook: "stop", DurationMS: 5},                             // pass, no enforcement
	} {
		if _, err := RecordHookMetricEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := SummarizeHookMetricsLog()
	if err != nil || !stats.OK {
		t.Fatalf("summarize failed: %+v err=%v", stats, err)
	}
	pre := stats.ByHook["pre-tool-use"]
	if pre.Blocks != 1 || pre.Asks != 1 {
		t.Fatalf("expected 1 block + 1 ask, got %+v", pre)
	}
	if pre.GateHitRate != 0.5 { // (1 block + 1 ask) / 4 invocations
		t.Fatalf("pre-tool-use gate_hit_rate want 0.5, got %v", pre.GateHitRate)
	}
	// A disabled gate: invocations continue but no interventions => rate 0.
	if stop := stats.ByHook["stop"]; stop.GateHitRate != 0 {
		t.Fatalf("stop gate_hit_rate want 0, got %v", stop.GateHitRate)
	}
	if stats.GateHitRate != 0.4 { // (1+1)/5 overall
		t.Fatalf("overall gate_hit_rate want 0.4, got %v", stats.GateHitRate)
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

func TestPruneHookMetricsLogKeepsNewestEntriesWithinLineLimit(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	base := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if _, err := RecordHookMetricEvent(HookMetricEvent{
			Timestamp:  base.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
			Hook:       "session-start",
			DurationMS: int64(10 + i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := pruneHookMetricsLog(720*time.Hour, hookMetricsPruneLimits{MaxEntries: 3})
	if err != nil || !result.OK || result.Pruned != 2 || result.Kept != 3 {
		t.Fatalf("unexpected bounded prune result: %+v err=%v", result, err)
	}
	events, err := readHookMetricEvents(HookMetricsLogPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 kept events, got %d", len(events))
	}
	if events[0].DurationMS != 12 || events[2].DurationMS != 14 {
		t.Fatalf("expected newest three events in original order, got %+v", events)
	}
}

func TestPruneHookMetricsLogKeepsNewestEntriesWithinByteLimit(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	base := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	events := []HookMetricEvent{
		{Timestamp: base.Format(time.RFC3339Nano), Hook: "session-start", Host: "old", DurationMS: 10},
		{Timestamp: base.Add(time.Second).Format(time.RFC3339Nano), Hook: "session-start", Host: "newer", DurationMS: 20},
		{Timestamp: base.Add(2 * time.Second).Format(time.RFC3339Nano), Hook: "session-start", Host: "newest", DurationMS: 30},
	}
	for _, event := range events {
		if _, err := RecordHookMetricEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	secondLine, err := json.Marshal(events[1])
	if err != nil {
		t.Fatal(err)
	}
	thirdLine, err := json.Marshal(events[2])
	if err != nil {
		t.Fatal(err)
	}
	maxBytes := int64(len(secondLine) + 1 + len(thirdLine) + 1)

	result, err := pruneHookMetricsLog(720*time.Hour, hookMetricsPruneLimits{MaxBytes: maxBytes})
	if err != nil || !result.OK || result.Pruned != 1 || result.Kept != 2 {
		t.Fatalf("unexpected byte-bounded prune result: %+v err=%v", result, err)
	}
	kept, err := readHookMetricEvents(HookMetricsLogPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 2 || kept[0].DurationMS != 20 || kept[1].DurationMS != 30 {
		t.Fatalf("expected newest two events within byte limit, got %+v", kept)
	}
}

func TestPruneHookMetricsLogSweepsOnlyStaleTempFiles(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if _, err := RecordHookMetricEvent(HookMetricEvent{Hook: "stop", DurationMS: 5}); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(stateDir, ".hook-metrics.jsonl-stale.tmp")
	fresh := filepath.Join(stateDir, ".hook-metrics.jsonl-fresh.tmp")
	if err := os.WriteFile(stale, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fresh, []byte("fresh"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	result, err := pruneHookMetricsLog(720*time.Hour, hookMetricsPruneLimits{StaleTempAge: time.Hour})
	if err != nil || !result.OK || result.StaleTempRemoved != 1 {
		t.Fatalf("unexpected stale temp sweep result: %+v err=%v", result, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp file should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh temp file should remain, err=%v", err)
	}
}

func TestPruneHookMetricsLogWaitsForMetricsLock(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if _, err := RecordHookMetricEvent(HookMetricEvent{Hook: "stop", DurationMS: 5}); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	if err := corestate.WithKeyLock(context.Background(), stateDir, "hook-metrics", func(context.Context) error {
		go func() {
			close(started)
			_, err := PruneHookMetricsLog(720 * time.Hour)
			done <- err
		}()
		<-started
		select {
		case err := <-done:
			t.Fatalf("prune should wait for hook-metrics lock, returned early with %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("prune after lock release: %v", err)
	}
}
