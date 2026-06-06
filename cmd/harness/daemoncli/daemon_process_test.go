package daemoncli

import (
	"path/filepath"
	"testing"
)

func TestStartDaemonProcessReturnsStartError(t *testing.T) {
	err := startDaemonProcess(filepath.Join(t.TempDir(), "missing-agent-harness"), daemonPaths{Dir: t.TempDir()})

	if err == nil {
		t.Fatal("expected missing executable to fail")
	}
}

type daemonStartFakeLock struct {
	closed bool
}

func (l *daemonStartFakeLock) Close() error {
	l.closed = true
	return nil
}
