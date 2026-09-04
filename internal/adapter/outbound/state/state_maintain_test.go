package state

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
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
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	t.Setenv("ISSUEOPS_WORKER_DIR", "")

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
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	t.Setenv("ISSUEOPS_WORKER_DIR", "")
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
	if _, err := os.Stat(filepath.Join(projectDir, "issueops.db")); !os.IsNotExist(err) {
		t.Fatalf("maintenance materialized project store: %v", err)
	}
}

func TestStateMaintainIgnoresSymlinkProjectNamespaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	t.Setenv("ISSUEOPS_WORKER_DIR", "")
	projectsDir := filepath.Join(stateDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	validDir := filepath.Join(projectsDir, "local")
	seedMaintainStore(t, validDir, "local", "value")
	outside := t.TempDir()
	seedMaintainStore(t, outside, "outside", "unchanged")
	wal := filepath.Join(outside, "issueops.db-wal")
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
	if err := os.Symlink(filepath.Join(outside, "issueops.db"), filepath.Join(fileLinked, "issueops.db")); err != nil {
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
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	t.Setenv("ISSUEOPS_WORKER_DIR", "")
	if err := os.WriteFile(filepath.Join(stateDir, "projects"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := StateMaintain()
	if err == nil || result.OK {
		t.Fatalf("expected project discovery error, result=%+v err=%v", result, err)
	}
}
