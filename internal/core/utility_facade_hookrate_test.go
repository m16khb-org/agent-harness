package core

import "testing"

// A2/G5: failure_rate = failures/invocations joins the failure log with the
// metrics invocation counter. The rate is computed only where invocations are
// known; the metrics Count denominator already counts every invocation.
func TestSummarizeHookFailureStatsJoinsRate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for range 4 {
		if err := RecordHookMetricEvent(HookMetricEvent{Hook: "pre-tool-use", DurationMS: 10}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := RecordHookFailureEvent(HookFailureEvent{Hook: "pre-tool-use", Error: "boom"}); err != nil {
		t.Fatal(err)
	}
	stats, err := SummarizeHookFailureStats()
	if err != nil || !stats.OK {
		t.Fatalf("summarize failed: %+v err=%v", stats, err)
	}
	if stats.Invocations["pre-tool-use"] != 4 {
		t.Fatalf("invocations want 4, got %+v", stats.Invocations)
	}
	if stats.FailureRate["pre-tool-use"] != 0.25 { // 1 failure / 4 invocations
		t.Fatalf("failure_rate want 0.25, got %v", stats.FailureRate["pre-tool-use"])
	}
	if stats.FailureRateOverall != 0.25 {
		t.Fatalf("overall failure_rate want 0.25, got %v", stats.FailureRateOverall)
	}
}

// A failure key with no recorded invocations (the "unparseable"-style case)
// must NOT appear in failure_rate as a misleading 0; its raw count stays
// visible via the embedded ByHook.
func TestSummarizeHookFailureStatsOmitsZeroInvocationKeys(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := RecordHookFailureEvent(HookFailureEvent{Hook: "ghost", Error: "x"}); err != nil {
		t.Fatal(err)
	}
	stats, err := SummarizeHookFailureStats()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stats.FailureRate["ghost"]; ok {
		t.Fatalf("ghost (0 invocations) must be omitted from failure_rate, got %+v", stats.FailureRate)
	}
	if stats.ByHook["ghost"] != 1 {
		t.Fatalf("ghost raw failure count should remain visible: %+v", stats.ByHook)
	}
}

// The freshly-made maps must not nil-panic on the empty-log path.
func TestSummarizeHookFailureStatsEmptyLogNoPanic(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stats, err := SummarizeHookFailureStats()
	if err != nil || !stats.OK {
		t.Fatalf("empty log must summarize cleanly: %+v err=%v", stats, err)
	}
	if len(stats.FailureRate) != 0 || stats.FailureRateOverall != 0 {
		t.Fatalf("empty log => no rates: %+v", stats)
	}
}
