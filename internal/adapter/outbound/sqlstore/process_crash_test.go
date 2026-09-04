package sqlstore

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

const (
	processHelperModeEnv = "ISSUEOPS_SQLSTORE_PROCESS_HELPER"
	processHelperDirEnv  = "ISSUEOPS_SQLSTORE_PROCESS_DIR"
)

type sqlstoreHelperProcess struct {
	cmd     *exec.Cmd
	markers chan string
	stderr  lockedBuffer
	done    chan struct{}

	mu      sync.Mutex
	waitErr error
}

type lockedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *lockedBuffer) text() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

func startSQLStoreHelper(t *testing.T, mode, dir string) *sqlstoreHelperProcess {
	t.Helper()
	h := &sqlstoreHelperProcess{
		cmd:     exec.Command(os.Args[0], "-test.run=^TestWithSpanProcessHelper$"),
		markers: make(chan string, 8),
		done:    make(chan struct{}),
	}
	h.cmd.Env = append(os.Environ(), processHelperModeEnv+"="+mode, processHelperDirEnv+"="+dir)
	h.cmd.Stderr = &h.stderr
	stdout, err := h.cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("%s StdoutPipe: %v", mode, err)
	}
	if err := h.cmd.Start(); err != nil {
		t.Fatalf("start %s helper: %v", mode, err)
	}
	t.Cleanup(func() { _ = h.killAndWait() })
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			h.markers <- scanner.Text()
		}
		close(h.markers)
	}()
	go func() {
		err := h.cmd.Wait()
		h.mu.Lock()
		h.waitErr = err
		h.mu.Unlock()
		close(h.done)
	}()
	return h
}

func (h *sqlstoreHelperProcess) killAndWait() error {
	select {
	case <-h.done:
	default:
		_ = h.cmd.Process.Kill()
		<-h.done
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.waitErr
}

func (h *sqlstoreHelperProcess) wait(timeout time.Duration) (error, bool) {
	select {
	case <-h.done:
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.waitErr, true
	case <-time.After(timeout):
		return nil, false
	}
}

func (h *sqlstoreHelperProcess) waitMarker(want string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case marker, ok := <-h.markers:
			if !ok {
				return fmt.Errorf("marker stream closed before %q", want)
			}
			if marker == want {
				return nil
			}
			return fmt.Errorf("marker=%q want=%q", marker, want)
		case <-timer.C:
			return fmt.Errorf("timed out waiting for %q", want)
		}
	}
}

func (h *sqlstoreHelperProcess) stderrText() string {
	return h.stderr.text()
}

func TestWithSpanProcessHelper(t *testing.T) {
	mode := os.Getenv(processHelperModeEnv)
	dir := os.Getenv(processHelperDirEnv)
	if mode == "" {
		t.Skip("subprocess helper only")
	}
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "holder":
		err = d.WithSpan(context.Background(), func(context.Context) error {
			fmt.Fprintln(os.Stdout, "locked")
			<-time.After(24 * time.Hour)
			return nil
		})
		t.Fatal(err)
	case "contender":
		fmt.Fprintln(os.Stdout, "attempting")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := d.WithSpan(ctx, func(context.Context) error {
			fmt.Fprintln(os.Stdout, "acquired")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
}

func TestWithSpanRecoversAfterHolderProcessIsKilled(t *testing.T) {
	dir := t.TempDir()
	holder := startSQLStoreHelper(t, "holder", dir)
	if err := holder.waitMarker("locked", 5*time.Second); err != nil {
		t.Fatalf("holder handshake: %v\nstderr:\n%s", err, holder.stderrText())
	}

	contender := startSQLStoreHelper(t, "contender", dir)
	if err := contender.waitMarker("attempting", 5*time.Second); err != nil {
		t.Fatalf("contender handshake: %v\nstderr:\n%s", err, contender.stderrText())
	}
	select {
	case marker, ok := <-contender.markers:
		if !ok {
			t.Fatalf("contender exited before holder kill\nholder stderr:\n%s\ncontender stderr:\n%s", holder.stderrText(), contender.stderrText())
		}
		select {
		case <-holder.done:
			t.Fatalf("contender marker before holder kill=%q; holder exited\nholder stderr:\n%s\ncontender stderr:\n%s", marker, holder.stderrText(), contender.stderrText())
		default:
			t.Fatalf("contender marker before holder kill=%q; holder still running\nholder stderr:\n%s\ncontender stderr:\n%s", marker, holder.stderrText(), contender.stderrText())
		}
	case <-time.After(200 * time.Millisecond):
	}

	_ = holder.killAndWait()
	if err := contender.waitMarker("acquired", 5*time.Second); err != nil {
		t.Fatalf("contender did not acquire after holder kill: %v\nstderr:\n%s", err, contender.stderrText())
	}
	if err, ok := contender.wait(5 * time.Second); !ok {
		t.Fatalf("contender did not exit\nstderr:\n%s", contender.stderrText())
	} else if err != nil {
		t.Fatalf("contender exit: %v\nstderr:\n%s", err, contender.stderrText())
	}
}
