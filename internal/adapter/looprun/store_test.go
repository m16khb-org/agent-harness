package looprun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/outbound/sqlstore"
)

func TestReadLoopRefusesFutureSchema(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	loop := startLoopForTest(t, "future-schema", 2)
	db, err := sqlstore.Open(StateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(loopBucket, loop.ID, []byte(`{"ok":true,"schema_version":99,"id":"`+loop.ID+`"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLoop(loop.ID); err == nil || !strings.Contains(err.Error(), "unsupported loop schema_version") {
		t.Fatalf("ReadLoop err=%v, want future schema refusal", err)
	}
}

func TestRepoGateSummaryDoesNotRepairExistingLoopStore(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := Start(StartLoopRequest{Repo: repo, Name: "read-only", Goal: "prove diagnostic reads stay read-only"}); err != nil {
		t.Fatal(err)
	}

	paths := []string{
		StateRoot(),
		filepath.Join(StateRoot(), "harness.db"),
		filepath.Join(StateRoot(), "harness.lock.db"),
	}
	for index, path := range paths {
		mode := os.FileMode(0o644)
		if index == 0 {
			mode = 0o755
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	summary, warnings := RepoGateSummaryFor(repo)
	if summary.Active != 1 || len(warnings) != 0 {
		t.Fatalf("loop summary=%#v warnings=%#v", summary, warnings)
	}
	for index, path := range paths {
		want := os.FileMode(0o644)
		if index == 0 {
			want = 0o755
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("loop diagnostic repaired %s mode to %o, want unchanged %o", path, got, want)
		}
	}
}
