// Package sqlstore is the shared SQLite-backed record store for harness state
// roots. Each state root directory owns two SQLite files: harness.db holds all
// records as (bucket, id, data-JSON) rows, and harness.lock.db exists only to
// carry the cross-process span lock. Read-modify-write spans serialize
// in-process on a per-directory mutex and cross-process by holding a BEGIN
// IMMEDIATE transaction on the lock database for the span's duration — the
// write lock dies with the process, so a crashed holder can never deadlock
// later contenders. Data writes autocommit on harness.db, so a span's own
// writes stay visible to it and to concurrent readers, matching the visibility
// the previous flock-based file layout had.
package sqlstore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	dataDBFile = "harness.db"
	spanDBFile = "harness.lock.db"
)

// DB is the handle for one state root directory.
type DB struct {
	dir  string
	data *sql.DB
	span *sql.DB
	mu   sync.Mutex
}

// Row is one record returned by GetAll.
type Row struct {
	ID   string
	Data []byte
}

var (
	handles   = map[string]*DB{}
	handlesMu sync.Mutex
)

// Open returns the cached handle for dir, creating the directory and both
// SQLite files when missing. Handles are cached per absolute directory so all
// callers in one process share the same in-process span mutex.
func Open(dir string) (*DB, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("sqlstore open %q: %w", dir, err)
	}
	handlesMu.Lock()
	defer handlesMu.Unlock()
	if d, ok := handles[abs]; ok {
		return d, nil
	}
	d, err := newDB(abs)
	if err != nil {
		return nil, err
	}
	handles[abs] = d
	return d, nil
}

// newDB opens an uncached handle. Tests use a second uncached handle to prove
// cross-process serialization comes from SQLite, not the shared mutex.
func newDB(abs string) (*DB, error) {
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, err
	}
	dataPath := filepath.Join(abs, dataDBFile)
	if err := touchPrivate(dataPath); err != nil {
		return nil, fmt.Errorf("sqlstore create data db: %w", err)
	}
	data, err := openSQLite(dataPath, "_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("sqlstore open data db: %w", err)
	}
	if _, err := data.Exec(`CREATE TABLE IF NOT EXISTS records (
		bucket TEXT NOT NULL,
		id     TEXT NOT NULL,
		data   BLOB NOT NULL,
		PRIMARY KEY (bucket, id)
	) WITHOUT ROWID`); err != nil {
		data.Close()
		return nil, fmt.Errorf("sqlstore init schema: %w", err)
	}
	spanPath := filepath.Join(abs, spanDBFile)
	if err := touchPrivate(spanPath); err != nil {
		data.Close()
		return nil, fmt.Errorf("sqlstore create span db: %w", err)
	}
	span, err := openSQLite(spanPath, "_pragma=busy_timeout(60000)&_txlock=immediate")
	if err != nil {
		data.Close()
		return nil, fmt.Errorf("sqlstore open span db: %w", err)
	}
	// The span database needs a schema write once so the file exists and BEGIN
	// IMMEDIATE has a real database to lock.
	if _, err := span.Exec(`CREATE TABLE IF NOT EXISTS span (id INTEGER PRIMARY KEY CHECK (id = 1))`); err != nil {
		data.Close()
		span.Close()
		return nil, fmt.Errorf("sqlstore init span db: %w", err)
	}
	return &DB{dir: abs, data: data, span: span}, nil
}

// touchPrivate pre-creates path with 0600 so SQLite (and its -wal/-shm
// sidecars, which inherit the database file's mode) never exposes state with
// wider permissions.
func touchPrivate(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func openSQLite(path, params string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?"+params)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// WithSpan serializes a read-modify-write span against all other spans on the
// same state root, in-process and cross-process. Spans must not nest — a
// nested span on the same directory self-deadlocks, exactly like the previous
// per-entity flock re-entry did. Multi-entity operations stay sequential
// single-span steps.
func (d *DB) WithSpan(fn func() error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.span.Begin()
	if err != nil {
		return fmt.Errorf("sqlstore span lock %s: %w", d.dir, err)
	}
	defer func() { _ = tx.Rollback() }()
	return fn()
}

// Get returns the record data for (bucket, id) and whether it exists.
func (d *DB) Get(bucket, id string) ([]byte, bool, error) {
	var data []byte
	err := d.data.QueryRow(`SELECT data FROM records WHERE bucket = ? AND id = ?`, bucket, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Put upserts the record data for (bucket, id).
func (d *DB) Put(bucket, id string, data []byte) error {
	_, err := d.data.Exec(`INSERT INTO records (bucket, id, data) VALUES (?, ?, ?)
		ON CONFLICT (bucket, id) DO UPDATE SET data = excluded.data`, bucket, id, data)
	return err
}

// Delete removes the record for (bucket, id); deleting an absent record is
// not an error.
func (d *DB) Delete(bucket, id string) error {
	_, err := d.data.Exec(`DELETE FROM records WHERE bucket = ? AND id = ?`, bucket, id)
	return err
}

// List returns the ids in bucket in ascending order.
func (d *DB) List(bucket string) ([]string, error) {
	rows, err := d.data.Query(`SELECT id FROM records WHERE bucket = ? ORDER BY id`, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAll returns every record in bucket ordered by id.
func (d *DB) GetAll(bucket string) ([]Row, error) {
	rows, err := d.data.Query(`SELECT id, data FROM records WHERE bucket = ? ORDER BY id`, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.Data); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteBucket removes every record in bucket.
func (d *DB) DeleteBucket(bucket string) error {
	_, err := d.data.Exec(`DELETE FROM records WHERE bucket = ?`, bucket)
	return err
}
