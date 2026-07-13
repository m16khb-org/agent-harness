package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-harness/internal/core/sqlstore"
)

func TestWithKeyLockPropagatesActiveRoot(t *testing.T) {
	dir := t.TempDir()
	err := WithKeyLock(context.Background(), dir, "outer", func(spanCtx context.Context) error {
		db, err := openStateDB(dir)
		if err != nil {
			return err
		}
		return db.WithSpanContext(spanCtx, func(context.Context) error { return nil })
	})
	var nested *sqlstore.NestedSpanError
	if !errors.As(err, &nested) {
		t.Fatalf("expected NestedSpanError, got %v", err)
	}
}

func TestWriteStateRecordRejectsKeyMismatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	// A caller-built record whose Key diverges from the write key would persist a
	// record StateRead later rejects; WriteStateRecord must reject it up front.
	if _, err := WriteStateRecord(dir, "foo", StateRecord{Key: "bar", SchemaVersion: StateCurrentSchemaVersion, Content: "x", Bytes: 1}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected key-mismatch error, got %v", err)
	}
	// Matching key (or empty Key) is accepted and round-trips.
	if _, err := WriteStateRecord(dir, "foo", StateRecord{Key: "foo", SchemaVersion: StateCurrentSchemaVersion, Content: "x", Bytes: 1}); err != nil {
		t.Fatalf("matching key should write: %v", err)
	}
	if read, err := StateRead("foo"); err != nil || read.Record.Content != "x" {
		t.Fatalf("round-trip failed: %q err=%v", read.Record.Content, err)
	}
}

func TestStateUpdateLockedReadModifyWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)

	// rec builds a valid persistable record; StateUpdate delegates record
	// construction (incl. Bytes, which StateRead validates) to the transform.
	rec := func(content string) StateRecord {
		return StateRecord{Key: "counter", SchemaVersion: StateCurrentSchemaVersion, Content: content, Bytes: len([]byte(content))}
	}

	// Create-from-absent: transform receives an empty record.
	res, err := StateUpdate("counter", func(cur StateRecord) (StateRecord, error) {
		if cur.Content != "" {
			t.Fatalf("expected absent record, got %q", cur.Content)
		}
		return rec("1"), nil
	})
	if err != nil || !res.OK {
		t.Fatalf("update create: ok=%v err=%v", res.OK, err)
	}
	if read, err := StateRead("counter"); err != nil || read.Record.Content != "1" {
		t.Fatalf("expected content 1, got %q err=%v", read.Record.Content, err)
	}

	// Real transform mutates + persists.
	if _, err := StateUpdate("counter", func(cur StateRecord) (StateRecord, error) {
		return rec(cur.Content + "2"), nil
	}); err != nil {
		t.Fatalf("update mutate: %v", err)
	}
	if read, err := StateRead("counter"); err != nil || read.Record.Content != "12" {
		t.Fatalf("expected content 12, got %q err=%v", read.Record.Content, err)
	}

	// Skip-write sentinel: an empty record returned by transform must NOT write.
	if res, err := StateUpdate("counter", func(cur StateRecord) (StateRecord, error) {
		return StateRecord{}, nil
	}); err != nil || !res.OK {
		t.Fatalf("update skip: ok=%v err=%v", res.OK, err)
	}
	if read, _ := StateRead("counter"); read.Record.Content != "12" {
		t.Fatalf("skip-write sentinel changed content: %q", read.Record.Content)
	}

	// Concurrent no-lost-update: the flock-serialized RMW must land all increments.
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = StateUpdate("ctr2", func(cur StateRecord) (StateRecord, error) {
				v := 0
				if cur.Content != "" {
					fmt.Sscanf(cur.Content, "%d", &v)
				}
				content := fmt.Sprintf("%d", v+1)
				return StateRecord{Key: "ctr2", SchemaVersion: StateCurrentSchemaVersion, Content: content, Bytes: len([]byte(content))}, nil
			})
		}()
	}
	wg.Wait()
	read, _ := StateRead("ctr2")
	var final int
	fmt.Sscanf(read.Record.Content, "%d", &final)
	if final != n {
		t.Fatalf("lost update under StateUpdate: expected %d, got %d", n, final)
	}
}

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
		lockErr <- withStateLock(context.Background(), dir, "locked-key", func(context.Context) error {
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
	writeRawStateRow(t, dir, "old", string(b)+"\n")

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

func TestStatePrunePrefixAppliesAgeAndCountOnlyToMatchingKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)

	write := func(key, updatedAt string) {
		t.Helper()
		if _, err := StateWrite(key, "content "+key); err != nil {
			t.Fatalf("StateWrite %s: %v", key, err)
		}
		read, err := StateRead(key)
		if err != nil {
			t.Fatalf("StateRead %s: %v", key, err)
		}
		read.Record.UpdatedAt = updatedAt
		b, err := json.MarshalIndent(read.Record, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", key, err)
		}
		writeRawStateRow(t, dir, key, string(b)+"\n")
	}

	write("external-llm-usage-old", "2000-01-01T00:00:00Z")
	write("external-llm-usage-recent-1", "2026-07-03T00:00:01Z")
	write("external-llm-usage-recent-2", "2026-07-03T00:00:02Z")
	write("external-llm-usage-recent-3", "2026-07-03T00:00:03Z")
	write("self-augment-lesson-old", "2000-01-01T00:00:00Z")

	result, err := StatePrunePrefix("external-llm-usage-", 365*24*time.Hour, 2, true)
	if err != nil {
		t.Fatalf("StatePrunePrefix: %v", err)
	}
	if !result.OK || !result.Confirm || !containsString(result.DeletedKeys, "external-llm-usage-old") || !containsString(result.DeletedKeys, "external-llm-usage-recent-1") {
		t.Fatalf("unexpected prefix prune result: %+v", result)
	}
	for _, key := range []string{"external-llm-usage-recent-2", "external-llm-usage-recent-3", "self-augment-lesson-old"} {
		if _, err := StateRead(key); err != nil {
			t.Fatalf("%s should be kept: %v", key, err)
		}
	}
	for _, key := range []string{"external-llm-usage-old", "external-llm-usage-recent-1"} {
		if _, err := StateRead(key); err == nil {
			t.Fatalf("%s should be pruned", key)
		}
	}
}

func TestStatePruneRejectsInvalidMaxAge(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := StatePrune(0, false); err == nil {
		t.Fatalf("StatePrune accepted zero max age")
	}
}
