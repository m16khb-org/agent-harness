package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)

	content := "seed=42\nLore: state roundtrip\n"
	written, err := StateWrite("checkpoint-1", content)
	if err != nil {
		t.Fatalf("StateWrite: %v", err)
	}
	if !written.OK {
		t.Fatalf("StateWrite ok=false: %+v", written)
	}
	if written.StateDir != dir {
		t.Fatalf("StateDir=%q want %q", written.StateDir, dir)
	}
	if written.Path != filepath.Join(dir, "checkpoint-1.json") {
		t.Fatalf("Path=%q", written.Path)
	}
	if written.Record.Bytes != len([]byte(content)) {
		t.Fatalf("Bytes=%d want %d", written.Record.Bytes, len([]byte(content)))
	}
	if written.Record.SchemaVersion != StateCurrentSchemaVersion {
		t.Fatalf("SchemaVersion=%d want %d", written.Record.SchemaVersion, StateCurrentSchemaVersion)
	}

	read, err := StateRead("checkpoint-1")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	if read.Record.Content != content {
		t.Fatalf("content=%q want %q", read.Record.Content, content)
	}

	listed, err := StateList()
	if err != nil {
		t.Fatalf("StateList: %v", err)
	}
	if len(listed.Keys) != 1 || listed.Keys[0] != "checkpoint-1" {
		t.Fatalf("Keys=%v", listed.Keys)
	}
	if len(listed.Records) != 1 || listed.Records[0].Bytes != len([]byte(content)) {
		t.Fatalf("Records=%+v", listed.Records)
	}
	if listed.Records[0].SchemaVersion != StateCurrentSchemaVersion {
		t.Fatalf("SchemaVersion=%d want %d", listed.Records[0].SchemaVersion, StateCurrentSchemaVersion)
	}
}

func TestStateWriteWaitsForKeyLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)

	locked := make(chan struct{})
	release := make(chan struct{})
	lockErr := make(chan error, 1)
	go func() {
		lockErr <- withStateLock(dir, "locked-key", func() error {
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

	started := make(chan struct{})
	writeDone := make(chan error, 1)
	go func() {
		close(started)
		_, err := StateWrite("locked-key", "locked content")
		writeDone <- err
	}()
	<-started

	select {
	case err := <-writeDone:
		t.Fatalf("StateWrite completed while key lock was held: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	if err := <-lockErr; err != nil {
		t.Fatalf("withStateLock: %v", err)
	}
	if err := <-writeDone; err != nil {
		t.Fatalf("StateWrite after lock release: %v", err)
	}
	read, err := StateRead("locked-key")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	if read.Record.Content != "locked content" {
		t.Fatalf("content=%q", read.Record.Content)
	}
}

func TestStateRejectsPathTraversalKeys(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, key := range []string{"", "../x", "x/y", "x\\y", "x..y"} {
		if _, err := StateWrite(key, "content"); err == nil {
			t.Fatalf("StateWrite(%q) succeeded; want error", key)
		}
	}
}

func TestStatePruneDryRunAndConfirm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	if _, err := StateWrite("old", "old content"); err != nil {
		t.Fatalf("StateWrite old: %v", err)
	}
	if _, err := StateWrite("fresh", "fresh content"); err != nil {
		t.Fatalf("StateWrite fresh: %v", err)
	}
	old, err := StateRead("old")
	if err != nil {
		t.Fatalf("StateRead old: %v", err)
	}
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	b, err := json.MarshalIndent(old.Record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old.Path, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	dry, err := StatePrune(time.Hour, false)
	if err != nil {
		t.Fatalf("StatePrune dry-run: %v", err)
	}
	if !dry.OK || !dry.DryRun || dry.Confirm || !containsString(dry.DeletedKeys, "old") || !containsString(dry.KeptKeys, "fresh") {
		t.Fatalf("unexpected dry-run prune result: %+v", dry)
	}
	if _, err := StateRead("old"); err != nil {
		t.Fatalf("dry-run removed old key: %v", err)
	}

	confirmed, err := StatePrune(time.Hour, true)
	if err != nil {
		t.Fatalf("StatePrune confirmed: %v", err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Confirm || !containsString(confirmed.DeletedKeys, "old") {
		t.Fatalf("unexpected confirmed prune result: %+v", confirmed)
	}
	if _, err := StateRead("old"); err == nil {
		t.Fatalf("old key still exists after confirmed prune")
	}
	if _, err := StateRead("fresh"); err != nil {
		t.Fatalf("fresh key removed unexpectedly: %v", err)
	}
}

func TestStatePruneRejectsInvalidMaxAge(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := StatePrune(0, false); err == nil {
		t.Fatalf("StatePrune accepted zero max age")
	}
}
