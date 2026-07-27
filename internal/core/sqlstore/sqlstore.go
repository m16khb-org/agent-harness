// Package sqlstore는 harness state root를 위한 공용 SQLite 기반 record store다.
// state root 디렉터리 하나는 SQLite 파일 두 개를 소유한다. harness.db는 모든
// record를 (bucket, id, data-JSON) row로 보관하고, harness.lock.db는 프로세스
// 간 span lock을 실어 나르기 위해서만 존재한다. read-modify-write span은
// 프로세스 안에서는 디렉터리별 token gate로, 프로세스 간에는 span이 지속되는
// 동안 lock 데이터베이스에 BEGIN IMMEDIATE 트랜잭션을 유지하는 방식으로
// 직렬화한다 — write lock은 프로세스와 함께 죽으므로, holder가 crash해도 이후
// 경쟁자가 deadlock에 빠질 수 없다. 데이터 write는 harness.db에서 autocommit
// 되므로 span 자신의 write가 그 span과 동시 reader에게 계속 보이며, 이는 이전
// flock 기반 파일 레이아웃이 가졌던 가시성과 동일하다. Apply는 여러 data row를
// 한 트랜잭션으로 commit해야 하는 호출자를 위한 좁은 예외다.
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
	// existingReadBusyTimeout은 read-only existing-store 조회가 일시적 SQLite
	// 경합(writer commit, daemon WAL checkpoint)에서 대기하는 시간의 상한이다.
	// 값이 0이면 밀리초 단위 checkpoint 구간 동안 lifecycle-hook 조회가 즉시
	// 실패해, 건강한 state에서도 mutation guard가 fail-closed됐다. 짧은 유한
	// 대기는 그런 허위 실패 없이 hook 응답성을 유지한다.
	existingReadBusyTimeout = 2 * time.Second
	openLockMaxWait         = 10 * time.Second
)

var sqliteFileSuffixes = [...]string{"", "-wal", "-shm", "-journal"}

// DB는 state root 디렉터리 하나에 대한 핸들이다.
type DB struct {
	dir      string
	data     *sql.DB
	span     *sql.DB
	spanGate chan struct{}
}

type spanChainKey struct{}

// NestedSpanError는 전파된 span chain에서 이미 활성인 root로 다시 진입하려는
// 시도를 보고한다.
type NestedSpanError struct {
	ActiveDirs   []string
	RequestedDir string
}

func (e *NestedSpanError) Error() string {
	return fmt.Sprintf("sqlstore nested span: root %q is already active in %v", e.RequestedDir, e.ActiveDirs)
}

// Row는 GetAll이 반환하는 record 하나다.
type Row struct {
	ID   string
	Data []byte
}

// SchemaObject는 기존 store의 non-internal SQLite schema object 하나다.
// maintenance 호출자는 state를 삭제하기 전에 이해하지 못하는 레이아웃을
// 거부하는 데 이를 사용한다.
type SchemaObject struct {
	Type  string
	Name  string
	Table string
	SQL   string
}

// ExistingLayout은 이미 존재하는 sqlstore root 하나의 read-only projection이다.
// bucket과 schema object는 결정적 순서로 반환된다.
type ExistingLayout struct {
	Buckets    []string
	DataSchema []SchemaObject
	SpanSchema []SchemaObject
}

// Mutation은 Apply 트랜잭션 안의 row upsert 또는 delete 하나다.
type Mutation struct {
	Bucket        string
	ID            string
	Data          []byte
	Delete        bool
	RequireAbsent bool
}

var (
	handles   = map[string]*DB{}
	handlesMu sync.Mutex
)

// Open은 dir에 대한 캐시된 핸들을 반환하며, 디렉터리와 두 SQLite 파일이 없으면
// 생성한다. 핸들은 절대 경로 디렉터리별로 캐시되므로 한 프로세스의 모든
// 호출자가 같은 in-process span mutex를 공유한다. 이미 제거된 root의 핸들은
// 다음 Open에서 닫고 축출해 임시 state root가 연결과 goroutine을 누적하지
// 않게 한다.
func Open(dir string) (*DB, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("sqlstore open %q: %w", dir, err)
	}
	handlesMu.Lock()
	defer handlesMu.Unlock()
	pruneRemovedHandlesLocked()
	if err := ensurePrivateRoot(abs); err != nil {
		return nil, fmt.Errorf("sqlstore secure root %s: %w", abs, err)
	}
	if _, err := repairPrivateSQLiteFiles(abs); err != nil {
		return nil, err
	}
	if d, ok := handles[abs]; ok {
		return d, nil
	}
	d, err := newDBWithRetry(abs)
	if err != nil {
		return nil, err
	}
	handles[abs] = d
	return d, nil
}

func pruneRemovedHandlesLocked() {
	for root, db := range handles {
		if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		delete(handles, root)
		_ = db.data.Close()
		_ = db.span.Close()
	}
}

// CloseRoot는 dir의 캐시된 핸들을 닫고 축출한다. 의도적으로 좁은 API다:
// 파괴적 maintenance는 이를 호출하기 전에 먼저 writer를 멈추고 진행 중인
// span을 끝내야 한다. 캐시되지 않은 root를 닫는 것은 no-op이다.
func CloseRoot(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("sqlstore close %q: %w", dir, err)
	}
	handlesMu.Lock()
	db, ok := handles[abs]
	if ok {
		delete(handles, abs)
	}
	handlesMu.Unlock()
	if !ok {
		return nil
	}
	return errors.Join(db.data.Close(), db.span.Close())
}

func newDBWithRetry(abs string) (*DB, error) {
	deadline := time.NewTimer(openLockMaxWait)
	defer deadline.Stop()
	retry := time.NewTicker(spanLockRetryGap)
	defer retry.Stop()
	for {
		db, err := newDB(abs)
		if err == nil || !isSQLiteLockContention(err) {
			return db, err
		}
		select {
		case <-deadline.C:
			return nil, err
		case <-retry.C:
		}
	}
}

// newDB는 캐시되지 않은 핸들을 연다. 테스트는 두 번째 uncached 핸들로 프로세스
// 간 직렬화가 공유 mutex가 아니라 SQLite에서 온다는 것을 증명한다.
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
	data, err := openSQLite(dataPath, "_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate")
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
	// SQLite busy handler는 driver가 blocked BEGIN을 interrupt할 때 즉시
	// 반환하지 않는다. 여기서는 비활성화하고, beginSpanTx가 60초 상한을 유지한
	// 채 호출자 context에 select할 수 있는 typed retry를 수행하게 한다.
	span, err := openSQLite(spanPath, "_pragma=busy_timeout(0)&_txlock=immediate")
	if err != nil {
		data.Close()
		return nil, fmt.Errorf("sqlstore open span db: %w", err)
	}
	// span 데이터베이스에는 schema write가 한 번은 필요하다. 그래야 파일이
	// 실제로 존재하고 BEGIN IMMEDIATE가 lock할 진짜 데이터베이스가 생긴다.
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

// touchPrivate는 path를 0600으로 미리 생성해, SQLite(와 데이터베이스 파일의
// mode를 물려받는 -wal/-shm sidecar)가 state를 더 넓은 권한으로 노출하지
// 못하게 한다.
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

// WithSpan은 root 하나의 read-modify-write span을 직렬화하고, 순서 있는
// active-root chain을 전파하며, 로컬 대기와 SQLite lock 대기 모두 ctx를
// 따르게 한다.
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

// Get은 (bucket, id)의 record 데이터와 존재 여부를 반환한다.
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

// GetExisting은 state root, data 데이터베이스, span 데이터베이스, schema를
// 생성하지 않고 primary-key row 하나를 읽는다. read-only 연결은 SQLite
// 경합에서 최대 existingReadBusyTimeout만 대기하므로, lifecycle-hook 조회는
// 일시적 writer commit과 WAL checkpoint를 견디면서도 유한하게 끝난다.
func GetExisting(dir, bucket, id string) ([]byte, bool, error) {
	data, err := openExistingData(dir)
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

// ListExisting은 state 파일을 생성하거나 복구하지 않고 기존 data store에서
// bucket의 ID를 반환한다. store가 없으면 fs.ErrNotExist를 반환한다.
func ListExisting(dir, bucket string) ([]string, error) {
	data, err := openExistingData(dir)
	if err != nil {
		return nil, err
	}
	defer data.Close()
	rows, err := data.Query(`SELECT id FROM records WHERE bucket = ? ORDER BY id`, bucket)
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

// GetAllExisting은 state 파일을 생성하거나 복구하지 않고 기존 data store에서
// bucket의 row를 반환한다. store가 없으면 fs.ErrNotExist를 반환한다.
func GetAllExisting(dir, bucket string) ([]Row, error) {
	data, err := openExistingData(dir)
	if err != nil {
		return nil, err
	}
	defer data.Close()
	rows, err := data.Query(`SELECT id, data FROM records WHERE bucket = ? ORDER BY id`, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Row{}
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.ID, &row.Data); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// InspectExisting은 state를 생성하거나 복구하지 않고 기존 store의 bucket과
// non-internal SQLite schema object를 보고한다.
func InspectExisting(dir string) (ExistingLayout, error) {
	data, err := openExistingData(dir)
	if err != nil {
		return ExistingLayout{}, err
	}
	defer data.Close()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ExistingLayout{}, fmt.Errorf("sqlstore existing inspect %q: %w", dir, err)
	}
	spanPath := filepath.Join(abs, spanDBFile)
	if _, err := os.Stat(spanPath); err != nil {
		if os.IsNotExist(err) {
			return ExistingLayout{}, fmt.Errorf("sqlstore existing span db %s: %w", abs, fs.ErrNotExist)
		}
		return ExistingLayout{}, err
	}
	span, err := openSQLite(spanPath, "mode=ro&_pragma=busy_timeout(0)&_pragma=query_only(1)")
	if err != nil {
		return ExistingLayout{}, err
	}
	defer span.Close()

	buckets, err := existingBuckets(data)
	if err != nil {
		return ExistingLayout{}, err
	}
	dataSchema, err := existingSchema(data)
	if err != nil {
		return ExistingLayout{}, err
	}
	spanSchema, err := existingSchema(span)
	if err != nil {
		return ExistingLayout{}, err
	}
	return ExistingLayout{Buckets: buckets, DataSchema: dataSchema, SpanSchema: spanSchema}, nil
}

func existingBuckets(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT bucket FROM records ORDER BY bucket`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	buckets := []string{}
	for rows.Next() {
		var bucket string
		if err := rows.Scan(&bucket); err != nil {
			return nil, err
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func existingSchema(db *sql.DB) ([]SchemaObject, error) {
	rows, err := db.Query(`SELECT type, name, tbl_name, COALESCE(sql, '')
		FROM sqlite_schema
		WHERE name NOT LIKE 'sqlite_%'
		ORDER BY type, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objects := []SchemaObject{}
	for rows.Next() {
		var object SchemaObject
		if err := rows.Scan(&object.Type, &object.Name, &object.Table, &object.SQL); err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}

func openExistingData(dir string) (*sql.DB, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("sqlstore existing open %q: %w", dir, err)
	}
	dataPath := filepath.Join(abs, dataDBFile)
	if _, err := os.Stat(dataPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("sqlstore existing data db %s: %w", abs, fs.ErrNotExist)
		}
		return nil, err
	}
	return openSQLite(dataPath, fmt.Sprintf("mode=ro&_pragma=busy_timeout(%d)&_pragma=query_only(1)", existingReadBusyTimeout/time.Millisecond))
}

// Put은 (bucket, id)의 record 데이터를 upsert한다.
func (d *DB) Put(bucket, id string, data []byte) error {
	_, err := d.data.Exec(`INSERT INTO records (bucket, id, data) VALUES (?, ?, ?)
		ON CONFLICT (bucket, id) DO UPDATE SET data = excluded.data`, bucket, id, data)
	return err
}

// Apply는 모든 mutation을 harness.db 트랜잭션 하나로 commit한다.
func (d *DB) Apply(ctx context.Context, mutations []Mutation) error {
	if len(mutations) == 0 {
		return nil
	}
	for _, mutation := range mutations {
		if mutation.Delete && mutation.RequireAbsent {
			return fmt.Errorf("sqlstore delete mutation cannot require an absent row")
		}
		if mutation.Bucket == "" || mutation.ID == "" {
			return fmt.Errorf("sqlstore mutation bucket and id are required")
		}
	}
	tx, err := d.data.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, mutation := range mutations {
		if mutation.RequireAbsent {
			var present int
			err := tx.QueryRowContext(ctx, `SELECT 1 FROM records WHERE bucket = ? AND id = ?`, mutation.Bucket, mutation.ID).Scan(&present)
			switch {
			case err == nil:
				return fmt.Errorf("sqlstore precondition failed: row %s/%s already exists", mutation.Bucket, mutation.ID)
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
		}
		if mutation.Delete {
			if _, err := tx.ExecContext(ctx, `DELETE FROM records WHERE bucket = ? AND id = ?`, mutation.Bucket, mutation.ID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO records (bucket, id, data) VALUES (?, ?, ?)
			ON CONFLICT (bucket, id) DO UPDATE SET data = excluded.data`, mutation.Bucket, mutation.ID, mutation.Data); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Delete는 (bucket, id)의 record를 제거한다. 없는 record를 삭제해도 오류가
// 아니다.
func (d *DB) Delete(bucket, id string) error {
	_, err := d.data.Exec(`DELETE FROM records WHERE bucket = ? AND id = ?`, bucket, id)
	return err
}

// List는 bucket의 id를 오름차순으로 반환한다.
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

// GetAll은 bucket의 모든 record를 id 순으로 반환한다.
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

// DeleteBucket은 bucket의 모든 record를 제거한다.
func (d *DB) DeleteBucket(bucket string) error {
	_, err := d.data.Exec(`DELETE FROM records WHERE bucket = ?`, bucket)
	return err
}
