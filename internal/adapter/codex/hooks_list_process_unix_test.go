//go:build unix

package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestListHooksTimeoutKillsAndReapsChild(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	workerRoot := t.TempDir()
	binDir := t.TempDir()
	pidPath := filepath.Join(binDir, "pid")
	childPIDPath := filepath.Join(binDir, "child-pid")
	script := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$$\" > " + shellQuoteForHooksListTest(pidPath) + "\nIFS= read -r _\n/bin/sleep 30 &\nchild=$!\nprintf '%s\\n' \"$child\" > " + shellQuoteForHooksListTest(childPIDPath) + "\nwait \"$child\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx := &deadlineSignalContext{done: make(chan struct{})}
	result := make(chan error, 1)
	go func() {
		_, err := ListHooks(ctx, workerRoot)
		result <- err
	}()
	waitForHooksListPIDFiles(t, ctx, result, pidPath, childPIDPath)
	started := time.Now()
	close(ctx.done)
	var err error
	select {
	case err = <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("Codex hooks/list did not return after its deadline")
	}
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timed out") {
		t.Fatalf("bounded parent deadline error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("timeout did not terminate promptly: %s", elapsed)
	}
	for _, path := range []string{pidPath, childPIDPath} {
		pidBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
		if err != nil {
			t.Fatal(err)
		}
		waitForHooksListProcessGone(t, pid)
	}
	audit, err := os.ReadFile(filepath.Join(stateDir, "audit", "process-execution.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(audit), `"outcome":"timeout"`) {
		t.Fatalf("timeout outcome missing from process audit: %s", audit)
	}
}

type deadlineSignalContext struct {
	done chan struct{}
}

func (*deadlineSignalContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *deadlineSignalContext) Done() <-chan struct{}   { return ctx.done }
func (*deadlineSignalContext) Value(any) any               { return nil }
func (ctx *deadlineSignalContext) Err() error {
	select {
	case <-ctx.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func waitForHooksListPIDFiles(t *testing.T, ctx *deadlineSignalContext, result <-chan error, paths ...string) {
	t.Helper()
	deadline := time.Now().Add(hooksListTimeout + time.Second)
	for {
		ready := true
		for _, path := range paths {
			if _, err := os.Stat(path); err != nil {
				ready = false
				break
			}
		}
		if ready {
			return
		}
		select {
		case err := <-result:
			t.Fatalf("Codex hooks/list exited before the timeout fixture started: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			close(ctx.done)
			<-result
			t.Fatal("Codex timeout fixture did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForHooksListProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if err != nil || time.Now().After(deadline) {
			t.Fatalf("Codex process tree member was not killed: pid=%d signal0=%v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func shellQuoteForHooksListTest(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
