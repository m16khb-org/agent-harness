package session

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestUnbindForCyclePreservesOtherCycleBinding reproduces the TOCTOU the
// per-repo lock closes: repo is bound to cycle B, then cycle A's done-unbind
// fires for the same repo. The compare-and-delete must observe CycleID==B and
// leave B's binding intact instead of dropping it.
func TestUnbindForCyclePreservesOtherCycleBinding(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	// Repo currently bound to cycle B (e.g. cycle B linked its worktree last).
	if err := Bind(store, "/repo/shared", "io-B", "2-b", "/wt/b"); err != nil {
		t.Fatal(err)
	}

	// Cycle A finishes and tries to unbind the shared repo binding.
	if err := UnbindForCycle(store, "/repo/shared", "io-A"); err != nil {
		t.Fatal(err)
	}

	// B's binding must survive: A's id did not match.
	b, err := Read(store, "/repo/shared")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-B" {
		t.Fatalf("expected B's binding to survive cross-cycle unbind, got %q", b.CycleID)
	}

	// The owning cycle B can still clear its own binding.
	if err := UnbindForCycle(store, "/repo/shared", "io-B"); err != nil {
		t.Fatal(err)
	}
	b, err = Read(store, "/repo/shared")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "" {
		t.Fatalf("expected binding cleared by owning cycle, got %q", b.CycleID)
	}
}

// TestUnbindForCycleMatchingRoundTrip is the bind/unbind round trip under the
// per-repo lock: a matching cycle id removes the binding.
func TestUnbindForCycleMatchingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "/repo/rt", "io-rt", "1-rt", "/wt/rt"); err != nil {
		t.Fatal(err)
	}
	if err := UnbindForCycle(store, "/repo/rt", "io-rt"); err != nil {
		t.Fatal(err)
	}
	b, err := Read(store, "/repo/rt")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "" {
		t.Fatalf("expected empty after matching unbind, got %q", b.CycleID)
	}

	// Unbind on an empty repo is a no-op (no binding, no matching id).
	if err := UnbindForCycle(store, "/repo/rt", "io-rt"); err != nil {
		t.Fatal(err)
	}
}

// TestSessionLockFilePersists asserts the per-repo lock file is created and is
// NOT deleted after the locked operation completes (flock inode invariant).
func TestSessionLockFilePersists(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "/repo/lk", "io-lk", "1-lk", "/wt/lk"); err != nil {
		t.Fatal(err)
	}
	want := lockPath(dir, bindingKey("/repo/lk"))
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected lock file to exist after bind: %v", err)
	}
	if filepath.Ext(want) != ".lock" {
		t.Fatalf("expected .lock extension, got %q", want)
	}

	// Unbind (also locked) must not remove the lock file.
	if err := Unbind(store, "/repo/lk"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("lock file must persist across unlock cycles: %v", err)
	}
}

// TestUnbindForCycleConcurrentNoOpKeepsBinding stresses the compare-and-delete
// under the lock: while many goroutines repeatedly fire a non-matching
// cross-cycle unbind, the owning binding must never be dropped.
func TestUnbindForCycleConcurrentNoOpKeepsBinding(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "/repo/cc", "io-owner", "1-cc", "/wt/cc"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if err := UnbindForCycle(store, "/repo/cc", "io-other"); err != nil {
					t.Errorf("unbind for non-matching cycle errored: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	b, err := Read(store, "/repo/cc")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-owner" {
		t.Fatalf("owning binding dropped under concurrent non-matching unbinds, got %q", b.CycleID)
	}
}

// TestUnbindForCycleVsConcurrentBindIsSerialized exercises the actual TOCTOU the
// per-repo lock closes: the owning cycle A's done-unbind (read CycleID -> delete)
// races a different cycle B's bind on the SAME repo. Under the per-repo lock the
// two operations serialize, so for every round the only reachable outcome is "B
// wins" — either A unbinds first and B then binds, or B binds first and A's
// compare-and-delete sees CycleID==B and does nothing. The torn outcome (A's
// delete dropping B's fresh binding, leaving the repo unbound) is exactly what
// an unlocked read-modify-write would allow, and must never happen here.
func TestUnbindForCycleVsConcurrentBindIsSerialized(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}
	const repo = "/repo/race"

	for round := 0; round < 64; round++ {
		if err := Bind(store, repo, "io-A", "1-a", "/wt/a"); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _ = UnbindForCycle(store, repo, "io-A") }()
		go func() { defer wg.Done(); _ = Bind(store, repo, "io-B", "2-b", "/wt/b") }()
		wg.Wait()

		b, err := Read(store, repo)
		if err != nil {
			t.Fatalf("round %d: read: %v", round, err)
		}
		if b.CycleID != "io-B" {
			t.Fatalf("round %d: B's bind must survive the race with A's unbind under the per-repo lock, got %q", round, b.CycleID)
		}
	}
}

// TestUnbindForCycleTrimsCycleID guards that the matching comparison uses the
// trimmed cycle id, mirroring Bind's trimming.
func TestUnbindForCycleTrimsCycleID(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "/repo/tr", "io-tr", "1-tr", "/wt/tr"); err != nil {
		t.Fatal(err)
	}
	if err := UnbindForCycle(store, "/repo/tr", "  io-tr  "); err != nil {
		t.Fatal(err)
	}
	if b, err := Read(store, "/repo/tr"); err != nil {
		t.Fatal(err)
	} else if strings.TrimSpace(b.CycleID) != "" {
		t.Fatalf("expected binding cleared with whitespace-padded id, got %q", b.CycleID)
	}
}
