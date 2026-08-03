package issueops

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
)

// TestWithIssueOpsLockRejectsSameRootReentry proves the in-process fail-closed
// guard: re-entering withIssueOpsLock on the same state root from inside an
// active span is rejected with NestedSpanError (self-deadlock prevention),
// while distinct roots remain composable.
func TestWithIssueOpsLockRejectsSameRootReentry(t *testing.T) {
	stateRoot := t.TempDir()
	outer := NewIssueOpsID(stateRoot, "outer")
	inner := NewIssueOpsID(stateRoot, "inner")

	err := withIssueOpsLock(context.Background(), stateRoot, outer, func(ctx context.Context) error {
		return withIssueOpsLock(ctx, stateRoot, inner, func(context.Context) error { return nil })
	})
	var nested *sqlstore.NestedSpanError
	if !errors.As(err, &nested) {
		t.Fatalf("same-root re-entry must be rejected with NestedSpanError, got %v", err)
	}
}

const (
	lockHelperModeEnv   = "HARNESS_ISSUEOPS_LOCK_HELPER"
	lockHelperRootEnv   = "HARNESS_ISSUEOPS_LOCK_ROOT"
	lockHelperMarkerEnv = "HARNESS_ISSUEOPS_LOCK_MARKER"
)

func appendLockMarker(path, line string) error {
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

// TestWithIssueOpsLockProcessHelper is the subprocess body for the cross-process
// mutual-exclusion test.
func TestWithIssueOpsLockProcessHelper(t *testing.T) {
	mode := os.Getenv(lockHelperModeEnv)
	if mode == "" {
		t.Skip("subprocess helper only")
	}
	root := os.Getenv(lockHelperRootEnv)
	marker := os.Getenv(lockHelperMarkerEnv)
	id := NewIssueOpsID(root, mode)
	switch mode {
	case "holder":
		if err := withIssueOpsLock(context.Background(), root, id, func(context.Context) error {
			if err := appendLockMarker(marker, "holder-locked"); err != nil {
				return err
			}
			time.Sleep(1200 * time.Millisecond)
			return appendLockMarker(marker, "holder-released")
		}); err != nil {
			t.Fatal(err)
		}
	case "contender":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := withIssueOpsLock(ctx, root, id, func(context.Context) error {
			return appendLockMarker(marker, "contender-acquired")
		}); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown lock helper mode %q", mode)
	}
}

func startLockHelper(t *testing.T, mode, root, marker string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWithIssueOpsLockProcessHelper$")
	cmd.Env = append(os.Environ(), lockHelperModeEnv+"="+mode, lockHelperRootEnv+"="+root, lockHelperMarkerEnv+"="+marker)
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

func readLockMarkers(path string) []string {
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

// TestWithIssueOpsLockSerializesAcrossProcesses proves the issueops cycle lock
// composes the sqlstore span into cross-process mutual exclusion on the same
// state root: the contender process can only acquire after the holder process
// releases, regardless of the different cycle ids.
func TestWithIssueOpsLockSerializesAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-process issueops lock test skipped in -short")
	}
	root := t.TempDir()
	marker := root + "/order.log"

	holder := startLockHelper(t, "holder", root, marker)
	deadline := time.Now().Add(5 * time.Second)
	for {
		lines := readLockMarkers(marker)
		if len(lines) >= 1 && lines[0] == "holder-locked" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("holder did not acquire the issueops lock in time; markers=%v", readLockMarkers(marker))
		}
		time.Sleep(20 * time.Millisecond)
	}

	contender := startLockHelper(t, "contender", root, marker)
	if err := contender.Wait(); err != nil {
		t.Fatalf("contender exited with error: %v", err)
	}
	if err := holder.Wait(); err != nil {
		t.Fatalf("holder exited with error: %v", err)
	}

	lines := readLockMarkers(marker)
	want := []string{"holder-locked", "holder-released", "contender-acquired"}
	if len(lines) != len(want) {
		t.Fatalf("unexpected marker order (contender may have acquired while holder held): got=%v want=%v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("issueops lock did not serialize across processes: got=%v want=%v", lines, want)
		}
	}
}
