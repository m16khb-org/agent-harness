package sqlstore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInspectExistingReportsBucketsAndSchemaWithoutMutation(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("session", "b", []byte(`{"value":2}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", "a", []byte(`{"value":1}`)); err != nil {
		t.Fatal(err)
	}

	before := snapshotSQLStoreFiles(t, dir)
	layout, err := InspectExisting(dir)
	if err != nil {
		t.Fatal(err)
	}
	after := snapshotSQLStoreFiles(t, dir)

	if !slices.Equal(layout.Buckets, []string{"issueops", "session"}) {
		t.Fatalf("buckets = %#v", layout.Buckets)
	}
	if len(layout.DataSchema) != 1 || layout.DataSchema[0].Type != "table" || layout.DataSchema[0].Name != "records" {
		t.Fatalf("data schema = %#v", layout.DataSchema)
	}
	if len(layout.SpanSchema) != 1 || layout.SpanSchema[0].Type != "table" || layout.SpanSchema[0].Name != "span" {
		t.Fatalf("span schema = %#v", layout.SpanSchema)
	}
	if !slices.Equal(before, after) {
		t.Fatalf("inspection mutated store files:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestInspectExistingMissingDoesNotCreateStore(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "missing")
	if _, err := InspectExisting(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("inspection created entries: %#v", entries)
	}
}

func TestCloseRootDropsCachedHandleAndAllowsFileRemoval(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", "a", []byte(`{"value":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := CloseRoot(dir); err != nil {
		t.Fatal(err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	handlesMu.Lock()
	_, cached := handles[abs]
	handlesMu.Unlock()
	if cached {
		t.Fatal("closed root remains cached")
	}
	if err := os.Remove(filepath.Join(dir, dataDBFile)); err != nil {
		t.Fatalf("remove closed data database: %v", err)
	}
	if err := CloseRoot(dir); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
}

func snapshotSQLStoreFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s:%s:%d", entry.Name(), info.Mode(), info.Size()))
	}
	return out
}
