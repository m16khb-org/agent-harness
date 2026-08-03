package state

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
)

func seedMaintainStore(t *testing.T, dir, id, value string) {
	t.Helper()
	db, err := sqlstore.Open(dir)
	if err != nil {
		t.Fatalf("open %s: %v", dir, err)
	}
	if err := db.Put("maintain", id, []byte(value)); err != nil {
		t.Fatalf("seed %s: %v", dir, err)
	}
}

func TestStateMaintainDiscoversLoopAndProjectStores(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")

	loopDir := filepath.Join(stateDir, "loop")
	projectA := filepath.Join(stateDir, "projects", "project-a")
	projectB := filepath.Join(stateDir, "projects", "project-b")
	for _, dir := range []string{loopDir, projectB, projectA} {
		seedMaintainStore(t, dir, "seed", strings.Repeat("x", 4096))
	}

	result, err := StateMaintain()
	if err != nil {
		t.Fatalf("StateMaintain: %v", err)
	}
	want := []string{loopDir, projectA, projectB}
	if len(result.Roots) != len(want) {
		t.Fatalf("maintained roots=%+v want %v", result.Roots, want)
	}
	for i, root := range result.Roots {
		if root.Dir != want[i] {
			t.Fatalf("root[%d]=%s want %s", i, root.Dir, want[i])
		}
		if !root.Checkpointed || root.WALBytesBefore == 0 || root.WALBytesAfter > 64 {
			t.Fatalf("root[%d] not checkpointed: %+v", i, root)
		}
	}
}

func TestStateMaintainDoesNotMaterializeLifecycleOnlyProjects(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	projectsDir := filepath.Join(stateDir, "projects")
	projectDir := filepath.Join(projectsDir, "profile-only")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	validDir := filepath.Join(projectsDir, "with-db")
	seedMaintainStore(t, validDir, "valid", "value")

	result, err := StateMaintain()
	if err != nil {
		t.Fatalf("StateMaintain: %v", err)
	}
	if len(result.Roots) != 1 || result.Roots[0].Dir != validDir {
		t.Fatalf("maintained roots=%+v want only %s", result.Roots, validDir)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "harness.db")); !os.IsNotExist(err) {
		t.Fatalf("maintenance materialized project store: %v", err)
	}
}

func TestStateMaintainIgnoresSymlinkProjectNamespaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	projectsDir := filepath.Join(stateDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	validDir := filepath.Join(projectsDir, "local")
	seedMaintainStore(t, validDir, "local", "value")
	outside := t.TempDir()
	seedMaintainStore(t, outside, "outside", "unchanged")
	wal := filepath.Join(outside, "harness.db-wal")
	before, err := os.Stat(wal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(projectsDir, "linked")); err != nil {
		t.Fatal(err)
	}
	fileLinked := filepath.Join(projectsDir, "file-linked")
	if err := os.MkdirAll(fileLinked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "harness.db"), filepath.Join(fileLinked, "harness.db")); err != nil {
		t.Fatal(err)
	}

	result, err := StateMaintain()
	if err != nil {
		t.Fatalf("StateMaintain: %v", err)
	}
	if len(result.Roots) != 1 || result.Roots[0].Dir != validDir {
		t.Fatalf("maintained roots=%+v want only %s", result.Roots, validDir)
	}
	for _, root := range result.Roots {
		if root.Dir == outside {
			t.Fatalf("followed project symlink: %+v", result.Roots)
		}
	}
	after, err := os.Stat(wal)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != before.Size() {
		t.Fatalf("outside WAL changed: %d -> %d", before.Size(), after.Size())
	}
}

func TestStateMaintainReportsProjectDiscoveryErrors(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	if err := os.WriteFile(filepath.Join(stateDir, "projects"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := StateMaintain()
	if err == nil || result.OK {
		t.Fatalf("expected project discovery error, result=%+v err=%v", result, err)
	}
}

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
	projectsPath := filepath.Join(dir, "projects")
	if err := os.WriteFile(projectsPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, ran2, err := MaybeMaintainStateStores(time.Hour)
	if err != nil || ran2 {
		t.Fatalf("second run within interval should be skipped: ran=%v err=%v", ran2, err)
	}
	if err := os.Remove(projectsPath); err != nil {
		t.Fatal(err)
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
