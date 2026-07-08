package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMaybeMaintainStateStoresAmortizes verifies the sentinel gate: the first
// call runs maintenance and touches the sentinel; an immediate second call is
// skipped because the sentinel's mtime is within the interval.
func TestMaybeMaintainStateStoresAmortizes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)

	// Materialize a store so StateMaintain has work to do.
	if _, err := StateWrite("maintain-test", "payload"); err != nil {
		t.Fatal(err)
	}

	// First run: not gated, executes maintenance.
	result, ran, err := MaybeMaintainStateStores(time.Hour)
	if err != nil || !ran {
		t.Fatalf("first run should execute: ran=%v err=%v result=%+v", ran, err, result)
	}
	if !result.OK {
		t.Fatalf("first run result not OK: %+v", result)
	}

	sentinel := filepath.Join(dir, ".last-store-maintain")
	info, err := os.Stat(sentinel)
	if err != nil {
		t.Fatalf("sentinel must exist after run: %v", err)
	}
	firstMtime := info.ModTime()

	// Second run within the interval: gated, skipped.
	_, ran2, err := MaybeMaintainStateStores(time.Hour)
	if err != nil || ran2 {
		t.Fatalf("second run within interval should be skipped: ran=%v err=%v", ran2, err)
	}

	// Sentinel mtime must not regress.
	info2, _ := os.Stat(sentinel)
	if info2.ModTime().Before(firstMtime) {
		t.Fatalf("sentinel mtime regressed: first=%v second=%v", firstMtime, info2.ModTime())
	}

	// Zero interval always runs (sentinel age is always >= 0).
	_, ran3, err := MaybeMaintainStateStores(0)
	if err != nil || !ran3 {
		t.Fatalf("zero-interval run should always execute: ran=%v err=%v", ran3, err)
	}
}
