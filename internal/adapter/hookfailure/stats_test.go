package hookfailure

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Q2 (quality program): hook failures were recorded but never aggregated —
// no failure-rate signal existed. SummarizeHookFailureLog turns the JSONL
// into the first measurable hook-health metric.
func TestSummarizeHookFailureLogAggregatesByHookAndRecency(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	for _, hook := range []string{"stop", "stop"} {
		if _, err := RecordHookFailureEvent(HookFailureEvent{Hook: hook, Error: "boom"}); err != nil {
			t.Fatal(err)
		}
	}
	// One old event (8 days ago) appended directly, since Record stamps now.
	old, err := json.Marshal(HookFailureEvent{
		Timestamp: time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano),
		Hook:      "pre-tool-use",
		Error:     "stale",
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(HookFailureLogPath(), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(old, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	stats, err := SummarizeHookFailureLog()
	if err != nil {
		t.Fatal(err)
	}
	if !stats.OK || stats.Total != 3 {
		t.Fatalf("expected 3 total failures, got %+v", stats)
	}
	if stats.ByHook["stop"] != 2 || stats.ByHook["pre-tool-use"] != 1 {
		t.Fatalf("unexpected by-hook counts: %+v", stats.ByHook)
	}
	if stats.Last24h != 2 || stats.Last7d != 2 {
		t.Fatalf("recency windows wrong: %+v", stats)
	}
	if stats.SizeBytes <= 0 || stats.Oldest == "" || stats.Newest == "" {
		t.Fatalf("log health fields missing: %+v", stats)
	}
}

func TestSummarizeHookFailureLogHandlesMissingLog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stats, err := SummarizeHookFailureLog()
	if err != nil || !stats.OK || stats.Total != 0 {
		t.Fatalf("missing log must summarize to zero: %+v err=%v", stats, err)
	}
}
