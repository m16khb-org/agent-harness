package sqlstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepeatedOpenKeepsHandleAndConnectionCountsStable(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.WithSpan(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.Get("resource", "missing"); err != nil {
		t.Fatal(err)
	}
	handlesMu.Lock()
	handlesBefore := len(handles)
	handlesMu.Unlock()
	dataBefore := d.data.Stats().OpenConnections
	spanBefore := d.span.Stats().OpenConnections
	fdBefore, fdOK := readableFDCount()

	for range 200 {
		again, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if again != d {
			t.Fatal("Open returned a different cached handle")
		}
		if _, _, err := again.Get("resource", "missing"); err != nil {
			t.Fatal(err)
		}
	}

	handlesMu.Lock()
	handlesAfter := len(handles)
	handlesMu.Unlock()
	if handlesAfter != handlesBefore {
		t.Fatalf("handles: %d -> %d", handlesBefore, handlesAfter)
	}
	if got := d.data.Stats().OpenConnections; got != dataBefore {
		t.Fatalf("data connections: %d -> %d", dataBefore, got)
	}
	if got := d.span.Stats().OpenConnections; got != spanBefore {
		t.Fatalf("span connections: %d -> %d", spanBefore, got)
	}
	if fdAfter, ok := readableFDCount(); fdOK && ok {
		t.Logf("/dev/fd: before=%d after=%d delta=%d", fdBefore, fdAfter, fdAfter-fdBefore)
	}
}

func TestOpenPrunesCachedHandlesForRemovedRoots(t *testing.T) {
	parent := t.TempDir()
	removed := filepath.Join(parent, "removed")
	live := filepath.Join(parent, "live")
	if _, err := Open(removed); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(removed); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(live); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = CloseRoot(live)
		_ = CloseRoot(removed)
	})

	removedAbs, err := filepath.Abs(removed)
	if err != nil {
		t.Fatal(err)
	}
	handlesMu.Lock()
	_, cached := handles[removedAbs]
	handlesMu.Unlock()
	if cached {
		t.Fatal("Open retained a cached handle after its state root was removed")
	}
}

func readableFDCount() (int, bool) {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return 0, false
	}
	return len(entries), true
}
