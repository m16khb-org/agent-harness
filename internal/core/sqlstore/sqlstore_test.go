package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGetExistingMissingDoesNotCreateStore(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "missing")
	if _, ok, err := GetExisting(dir, "b", "x"); !errors.Is(err, fs.ErrNotExist) || ok {
		t.Fatalf("missing existing-store read: ok=%v err=%v", ok, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("existing-store read created files or directories: %v", entries)
	}
}

func TestListAndGetAllExistingDoNotCreateMissingStore(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "missing")
	if _, err := ListExisting(dir, "b"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing existing-store list error = %v", err)
	}
	if _, err := GetAllExisting(dir, "b"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing existing-store get-all error = %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("existing-store collection created files or directories: %v", entries)
	}
}

func TestListAndGetAllExistingReadSortedRows(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("b", "z", []byte(`{"value":2}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("b", "a", []byte(`{"value":1}`)); err != nil {
		t.Fatal(err)
	}

	ids, err := ListExisting(dir, "b")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := GetAllExisting(dir, "b")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []string{"a", "z"}) {
		t.Fatalf("existing ids = %#v", ids)
	}
	if len(rows) != 2 || rows[0].ID != "a" || string(rows[0].Data) != `{"value":1}` || rows[1].ID != "z" {
		t.Fatalf("existing rows = %#v", rows)
	}
}

func TestGetExistingReturnsBoundedErrorWhileDataStoreIsContended(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, dataDBFile)
	if err := touchPrivate(dataPath); err != nil {
		t.Fatal(err)
	}
	dataDB, err := openSQLite(dataPath, "_pragma=busy_timeout(0)&_pragma=journal_mode(DELETE)")
	if err != nil {
		t.Fatal(err)
	}
	defer dataDB.Close()
	if _, err := dataDB.Exec(`CREATE TABLE records (bucket TEXT NOT NULL, id TEXT NOT NULL, data BLOB NOT NULL, PRIMARY KEY (bucket, id)) WITHOUT ROWID`); err != nil {
		t.Fatal(err)
	}
	if _, err := dataDB.Exec(`INSERT INTO records (bucket, id, data) VALUES ('b', 'x', '{"v":1}')`); err != nil {
		t.Fatal(err)
	}
	conn, err := dataDB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `UPDATE records SET data = '{"v":2}' WHERE bucket = 'b' AND id = 'x'`); err != nil {
		t.Fatal(err)
	}

	type lookupResult struct {
		data []byte
		ok   bool
		err  error
	}
	lookupDone := make(chan lookupResult, 1)
	go func() {
		data, ok, err := GetExisting(dir, "b", "x")
		lookupDone <- lookupResult{data: data, ok: ok, err: err}
	}()

	select {
	case result := <-lookupDone:
		if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(fmt.Sprint(result.err))
		if result.err == nil || (!strings.Contains(lower, "locked") && !strings.Contains(lower, "busy")) {
			t.Fatalf("contended existing-store read must return a bounded lock error: data=%q ok=%v err=%v", result.data, result.ok, result.err)
		}
	case <-time.After(2 * time.Second):
		if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
			t.Fatal(err)
		}
		result := <-lookupDone
		t.Fatalf("existing-store read waited for contended data-store release: data=%q ok=%v err=%v", result.data, result.ok, result.err)
	}
}

func TestOpenCreatesDBAndIsCachedPerDir(t *testing.T) {
	dir := t.TempDir()
	d1, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	d2, err := Open(dir)
	if err != nil {
		t.Fatalf("Open twice: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("expected cached handle for same dir")
	}
	if _, err := os.Stat(filepath.Join(dir, "harness.db")); err != nil {
		t.Fatalf("expected data db file: %v", err)
	}
}

func TestPutGetDeleteRoundtrip(t *testing.T) {
	d := openTestDB(t)
	if _, ok, err := d.Get("b", "x"); err != nil || ok {
		t.Fatalf("expected absent record, ok=%v err=%v", ok, err)
	}
	if err := d.Put("b", "x", []byte(`{"v":1}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	data, ok, err := d.Get("b", "x")
	if err != nil || !ok || string(data) != `{"v":1}` {
		t.Fatalf("Get after Put: data=%q ok=%v err=%v", data, ok, err)
	}
	if err := d.Put("b", "x", []byte(`{"v":2}`)); err != nil {
		t.Fatalf("Put overwrite: %v", err)
	}
	data, _, _ = d.Get("b", "x")
	if string(data) != `{"v":2}` {
		t.Fatalf("expected overwrite, got %q", data)
	}
	if err := d.Delete("b", "x"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := d.Get("b", "x"); ok {
		t.Fatalf("expected record deleted")
	}
	if err := d.Delete("b", "x"); err != nil {
		t.Fatalf("Delete absent should be nil, got %v", err)
	}
}

func TestBucketsAreIsolatedAndListSorted(t *testing.T) {
	d := openTestDB(t)
	for _, id := range []string{"c", "a", "b"} {
		if err := d.Put("one", id, []byte("1")); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	if err := d.Put("two", "z", []byte("2")); err != nil {
		t.Fatalf("Put other bucket: %v", err)
	}
	ids, err := d.List("one")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 3 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Fatalf("expected sorted [a b c], got %v", ids)
	}
	rows, err := d.GetAll("one")
	if err != nil || len(rows) != 3 || rows[0].ID != "a" || string(rows[2].Data) != "1" {
		t.Fatalf("GetAll: rows=%v err=%v", rows, err)
	}
	if _, ok, _ := d.Get("two", "a"); ok {
		t.Fatalf("bucket isolation broken")
	}
	if err := d.DeleteBucket("one"); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
	ids, _ = d.List("one")
	if len(ids) != 0 {
		t.Fatalf("expected empty bucket after DeleteBucket, got %v", ids)
	}
	if _, ok, _ := d.Get("two", "z"); !ok {
		t.Fatalf("DeleteBucket must not touch other buckets")
	}
}

func TestWithSpanSerializesAcrossHandles(t *testing.T) {
	// Two independent handles simulate two processes: serialization must come
	// from the sqlite span lock, not the in-process mutex.
	dir := t.TempDir()
	d1, err := newDB(dir)
	if err != nil {
		t.Fatalf("newDB: %v", err)
	}
	d2, err := newDB(dir)
	if err != nil {
		t.Fatalf("newDB second handle: %v", err)
	}
	inSpan := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- d1.WithSpan(context.Background(), func(context.Context) error {
			close(inSpan)
			<-release
			return nil
		})
	}()
	<-inSpan
	entered := make(chan struct{})
	go func() {
		_ = d2.WithSpan(context.Background(), func(context.Context) error {
			close(entered)
			return nil
		})
	}()
	select {
	case <-entered:
		t.Fatalf("second span entered while first still held")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first span: %v", err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatalf("second span never entered after release")
	}
}

func TestWithSpanAllowsWritesInsideSpanAndPropagatesError(t *testing.T) {
	d := openTestDB(t)
	err := d.WithSpan(context.Background(), func(context.Context) error {
		if err := d.Put("b", "k", []byte("v")); err != nil {
			return err
		}
		data, ok, err := d.Get("b", "k")
		if err != nil || !ok || string(data) != "v" {
			return fmt.Errorf("own write not visible inside span: %q %v %v", data, ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithSpan: %v", err)
	}
	wantErr := fmt.Errorf("boom")
	if err := d.WithSpan(context.Background(), func(context.Context) error { return wantErr }); err != wantErr {
		t.Fatalf("expected callback error propagated, got %v", err)
	}
}

func TestConcurrentSpansAndReadsRace(t *testing.T) {
	d := openTestDB(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("id-%d", n%4)
			for j := 0; j < 10; j++ {
				_ = d.WithSpan(context.Background(), func(context.Context) error {
					_, _, _ = d.Get("race", id)
					return d.Put("race", id, []byte(fmt.Sprintf("%d-%d", n, j)))
				})
				_, _, _ = d.Get("race", id)
			}
		}(i)
	}
	wg.Wait()
	ids, err := d.List("race")
	if err != nil || len(ids) != 4 {
		t.Fatalf("expected 4 ids, got %v err=%v", ids, err)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return d
}
