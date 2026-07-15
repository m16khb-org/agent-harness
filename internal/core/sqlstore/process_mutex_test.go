package sqlstore

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	mutexHelperModeEnv   = "HARNESS_SQLSTORE_MUTEX_HELPER"
	mutexHelperDirEnv    = "HARNESS_SQLSTORE_MUTEX_DIR"
	mutexHelperMarkerEnv = "HARNESS_SQLSTORE_MUTEX_MARKER"
)

// appendOrderedMarker appends a single line to the shared ordered-marker file
// with an fsync, so a second OS process observing the file sees a total order.
func appendOrderedMarker(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// TestWithSpanMutexHelper is the subprocess body for the cross-process mutual
// exclusion test. It is a no-op unless invoked as a helper (env set).
func TestWithSpanMutexHelper(t *testing.T) {
	mode := os.Getenv(mutexHelperModeEnv)
	if mode == "" {
		t.Skip("subprocess helper only")
	}
	dir := os.Getenv(mutexHelperDirEnv)
	marker := os.Getenv(mutexHelperMarkerEnv)
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "holder":
		if err := d.WithSpan(context.Background(), func(context.Context) error {
			if err := appendOrderedMarker(marker, "holder-locked"); err != nil {
				return err
			}
			// Hold long enough that the contender provably blocks on BEGIN
			// IMMEDIATE while the span is held, then release normally.
			time.Sleep(1200 * time.Millisecond)
			return appendOrderedMarker(marker, "holder-released")
		}); err != nil {
			t.Fatal(err)
		}
	case "contender":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := d.WithSpan(ctx, func(context.Context) error {
			return appendOrderedMarker(marker, "contender-acquired")
		}); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown mutex helper mode %q", mode)
	}
}

func startMutexHelper(t *testing.T, mode, dir, marker string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWithSpanMutexHelper$")
	cmd.Env = append(os.Environ(), mutexHelperModeEnv+"="+mode, mutexHelperDirEnv+"="+dir, mutexHelperMarkerEnv+"="+marker)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s helper: %v", mode, err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd
}

func readMarkerLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// TestWithSpanMutualExclusionAcrossProcesses proves BEGIN IMMEDIATE serializes
// spans across two OS processes: while the holder process owns the span, the
// contender blocks and can only acquire after the holder releases normally. The
// shared ordered-marker file records the total order.
func TestWithSpanMutualExclusionAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-process span mutual-exclusion test skipped in -short")
	}
	dir := t.TempDir()
	marker := dir + "/order.log"

	holder := startMutexHelper(t, "holder", dir, marker)
	// Wait until the holder is provably inside the span.
	deadline := time.Now().Add(5 * time.Second)
	for {
		lines := readMarkerLines(marker)
		if len(lines) >= 1 && lines[0] == "holder-locked" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("holder did not acquire the span in time; markers=%v", readMarkerLines(marker))
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Start the contender while the holder still owns the span. It must block.
	contender := startMutexHelper(t, "contender", dir, marker)
	if err := contender.Wait(); err != nil {
		t.Fatalf("contender exited with error: %v", err)
	}
	if err := holder.Wait(); err != nil {
		t.Fatalf("holder exited with error: %v", err)
	}

	lines := readMarkerLines(marker)
	want := []string{"holder-locked", "holder-released", "contender-acquired"}
	if len(lines) != len(want) {
		t.Fatalf("unexpected marker order (contender may have acquired while holder held): got=%v want=%v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("marker order violated mutual exclusion: got=%v want=%v", lines, want)
		}
	}
}
