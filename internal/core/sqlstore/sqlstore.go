// Package sqlstore is the shared SQLite-backed record store for harness state
// roots. Each state root directory owns two SQLite files: harness.db holds all
// records as (bucket, id, data-JSON) rows, and harness.lock.db exists only to
// carry the cross-process span lock. Read-modify-write spans serialize
// in-process on a per-directory token gate and cross-process by holding a BEGIN
// IMMEDIATE transaction on the lock database for the span's duration — the
// write lock dies with the process, so a crashed holder can never deadlock
// later contenders. Data writes autocommit on harness.db, so a span's own
// writes stay visible to it and to concurrent readers, matching the visibility
// the previous flock-based file layout had.
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	dataDBFile       = "harness.db"
	spanDBFile       = "harness.lock.db"
	spanLockMaxWait  = 60 * time.Second
	spanLockRetryGap = 10 * time.Millisecond
)

var sqliteFileSuffixes = [...]string{"", "-wal", "-shm", "-journal"}

// DB is the handle for one state root directory.
type DB struct {
	dir      string
	data     *sql.DB
	span     *sql.DB
	spanGate chan struct{}
}

type spanChainKey struct{}

// NestedSpanError reports an attempted re-entry into a root that is already
// active in the propagated span chain.
type NestedSpanError struct {
	ActiveDirs   []string
	RequestedDir string
}

func (e *NestedSpanError) Error() string {
	return fmt.Sprintf("sqlstore nested span: root %q is already active in %v", e.RequestedDir, e.ActiveDirs)
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
	if err := ensurePrivateRoot(abs); err != nil {
		return nil, fmt.Errorf("sqlstore secure root %s: %w", abs, err)
	}
	if _, err := repairPrivateSQLiteFiles(abs); err != nil {
		return nil, err
	}
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
	if err := ensurePrivateRoot(abs); err != nil {
		return nil, fmt.Errorf("sqlstore secure root %s: %w", abs, err)
	}
	if _, err := repairPrivateSQLiteFiles(abs); err != nil {
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
	// SQLite's busy handler does not return promptly when the driver interrupts
	// a blocked BEGIN. Disable it here and let beginSpanTx perform typed retries
	// that can select on the caller context while preserving the 60-second cap.
	span, err := openSQLite(spanPath, "_pragma=busy_timeout(0)&_txlock=immediate")
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
	if _, err := repairPrivateSQLiteFiles(abs); err != nil {
		data.Close()
		span.Close()
		return nil, err
	}
	return &DB{dir: abs, data: data, span: span, spanGate: newSpanGate()}, nil
}

func newSpanGate() chan struct{} {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return gate
}

// touchPrivate pre-creates path with 0600 so SQLite (and its -wal/-shm
// sidecars, which inherit the database file's mode) never exposes state with
// wider permissions.
func touchPrivate(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular SQLite file %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
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

func ensurePrivateRoot(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing non-directory state root %s", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func repairPrivateSQLiteFiles(dir string) ([]string, error) {
	var repaired []string
	for _, base := range [...]string{dataDBFile, spanDBFile} {
		for _, suffix := range sqliteFileSuffixes {
			name := base + suffix
			path := filepath.Join(dir, name)
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("sqlstore inspect permissions %s: %w", path, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("sqlstore refusing non-regular SQLite file %s", path)
			}
			if info.Mode().Perm() == 0o600 {
				continue
			}
			if err := os.Chmod(path, 0o600); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("sqlstore chmod %s: %w", path, err)
			}
			repaired = append(repaired, name)
		}
	}
	return repaired, nil
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

// WithSpan serializes a read-modify-write span for one root, propagates the
// ordered active-root chain, and makes both local and SQLite lock waits obey ctx.
func (d *DB) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	if ctx == nil {
		return fmt.Errorf("sqlstore span context is required")
	}
	if fn == nil {
		return fmt.Errorf("sqlstore span callback is required")
	}
	chain, _ := ctx.Value(spanChainKey{}).([]string)
	chain = append([]string(nil), chain...)
	for _, active := range chain {
		if active == d.dir {
			return &NestedSpanError{
				ActiveDirs:   append([]string(nil), chain...),
				RequestedDir: d.dir,
			}
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.spanGate:
	}
	defer func() { d.spanGate <- struct{}{} }()
	tx, err := d.beginSpanTx(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("sqlstore span lock %s: %w", d.dir, ctxErr)
		}
		return fmt.Errorf("sqlstore span lock %s: %w", d.dir, err)
	}
	defer func() { _ = tx.Rollback() }()
	spanCtx := context.WithValue(ctx, spanChainKey{}, append(chain, d.dir))
	return fn(spanCtx)
}

func (d *DB) beginSpanTx(ctx context.Context) (*sql.Tx, error) {
	maxWait := time.NewTimer(spanLockMaxWait)
	defer maxWait.Stop()
	retry := time.NewTicker(spanLockRetryGap)
	defer retry.Stop()

	for {
		tx, err := d.span.BeginTx(ctx, nil)
		if err == nil {
			return tx, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if !isSQLiteLockContention(err) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-maxWait.C:
			return nil, err
		case <-retry.C:
		}
	}
}

func isSQLiteLockContention(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	primaryCode := sqliteErr.Code() & 0xff
	return primaryCode == int(sqlite3.SQLITE_BUSY) || primaryCode == int(sqlite3.SQLITE_LOCKED)
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

// GetExisting reads one primary-key row without creating a state root, data
// database, span database, or schema. Its read-only connection never waits on
// SQLite contention, which keeps lifecycle-hook lookups bounded.
func GetExisting(dir, bucket, id string) ([]byte, bool, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, false, fmt.Errorf("sqlstore existing open %q: %w", dir, err)
	}
	dataPath := filepath.Join(abs, dataDBFile)
	if _, err := os.Stat(dataPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, fmt.Errorf("sqlstore existing data db %s: %w", abs, fs.ErrNotExist)
		}
		return nil, false, err
	}
	data, err := openSQLite(dataPath, "mode=ro&_pragma=busy_timeout(0)&_pragma=query_only(1)")
	if err != nil {
		return nil, false, err
	}
	defer data.Close()
	var raw []byte
	err = data.QueryRow(`SELECT data FROM records WHERE bucket = ? AND id = ?`, bucket, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
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
