package workpool

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/core/state"
)

// poolBucket holds one row per pool; task rows live in a per-pool bucket so
// task ids stay scoped to their pool exactly like the old per-pool directory.
const poolBucket = "pool"

func taskBucket(poolID string) string {
	return "task:" + poolID
}

func StateRoot() string {
	return filepath.Join(state.StateDir(), "workpool")
}

func openStore() (*sqlstore.DB, error) {
	return sqlstore.Open(StateRoot())
}

func ReadPool(poolID string) (WorkPool, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return WorkPool{OK: false}, err
	}
	db, err := openStore()
	if err != nil {
		return WorkPool{OK: false, ID: poolID}, err
	}
	data, ok, err := db.Get(poolBucket, poolID)
	if err != nil {
		return WorkPool{OK: false, ID: poolID}, err
	}
	if !ok {
		return WorkPool{OK: false, ID: poolID}, fmt.Errorf("workpool %s: %w", poolID, fs.ErrNotExist)
	}
	var pool WorkPool
	if err := json.Unmarshal(data, &pool); err != nil {
		return WorkPool{OK: false, ID: poolID}, err
	}
	if pool.ID != poolID {
		return WorkPool{OK: false, ID: poolID}, fmt.Errorf("workpool id mismatch: record has %q", pool.ID)
	}
	if err := normalizePoolSchemaVersion(&pool); err != nil {
		return WorkPool{OK: false, ID: poolID}, err
	}
	pool.OK = true
	return pool, nil
}

func writePool(pool WorkPool) (WorkPool, error) {
	if _, err := normalizePoolID(pool.ID); err != nil {
		pool.OK = false
		return pool, err
	}
	if err := normalizePoolSchemaVersion(&pool); err != nil {
		pool.OK = false
		return pool, err
	}
	db, err := openStore()
	if err != nil {
		pool.OK = false
		return pool, err
	}
	pool.OK = true
	data, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		pool.OK = false
		return pool, err
	}
	if err := db.Put(poolBucket, pool.ID, data); err != nil {
		pool.OK = false
		return pool, err
	}
	return pool, nil
}

// ListPoolIDs returns every pool id in ascending order.
func ListPoolIDs() ([]string, error) {
	db, err := openStore()
	if err != nil {
		return nil, err
	}
	return db.List(poolBucket)
}

func ReadTask(poolID, taskID string) (WorkTask, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return WorkTask{OK: false}, err
	}
	taskID, err = normalizeTaskID(taskID)
	if err != nil {
		return WorkTask{OK: false, PoolID: poolID}, err
	}
	db, err := openStore()
	if err != nil {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, err
	}
	data, ok, err := db.Get(taskBucket(poolID), taskID)
	if err != nil {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, err
	}
	if !ok {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, fmt.Errorf("worktask %s/%s: %w", poolID, taskID, fs.ErrNotExist)
	}
	var task WorkTask
	if err := json.Unmarshal(data, &task); err != nil {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, err
	}
	if task.ID != taskID || task.PoolID != poolID {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, fmt.Errorf("worktask id mismatch: record has %q/%q", task.PoolID, task.ID)
	}
	if err := normalizeTaskSchemaVersion(&task); err != nil {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, err
	}
	task.OK = true
	return task, nil
}

func writeTask(task WorkTask) (WorkTask, error) {
	poolID, err := normalizePoolID(task.PoolID)
	if err != nil {
		task.OK = false
		return task, err
	}
	if _, err := normalizeTaskID(task.ID); err != nil {
		task.OK = false
		return task, err
	}
	if err := normalizeTaskSchemaVersion(&task); err != nil {
		task.OK = false
		return task, err
	}
	db, err := openStore()
	if err != nil {
		task.OK = false
		return task, err
	}
	task.OK = true
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		task.OK = false
		return task, err
	}
	if err := db.Put(taskBucket(poolID), task.ID, data); err != nil {
		task.OK = false
		return task, err
	}
	return task, nil
}

func ListTasks(poolID string) ([]WorkTask, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return nil, err
	}
	ids, err := taskFileIDs(poolID)
	if err != nil {
		return nil, err
	}
	tasks := make([]WorkTask, 0, len(ids))
	for _, id := range ids {
		task, err := ReadTask(poolID, id)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// taskFileIDs returns the pool's task ids in ascending order.
func taskFileIDs(poolID string) ([]string, error) {
	db, err := openStore()
	if err != nil {
		return nil, err
	}
	return db.List(taskBucket(poolID))
}

func newPoolID(repo, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(name)))
	return "wp-" + hex.EncodeToString(sum[:])[:12]
}

func newTaskID(seq int, title string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(title)))
	return fmt.Sprintf("task-%04d-%s", seq, hex.EncodeToString(sum[:])[:8])
}

func normalizePoolID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("pool_id is required")
	}
	if !strings.HasPrefix(id, "wp-") || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid workpool id %q", id)
	}
	return id, nil
}

func normalizeTaskID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("task_id is required")
	}
	if !strings.HasPrefix(id, "task-") || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid worktask id %q", id)
	}
	return id, nil
}

func normalizePoolSchemaVersion(pool *WorkPool) error {
	switch {
	case pool.SchemaVersion == 0:
		pool.SchemaVersion = WorkPoolCurrentSchemaVersion
		return nil
	case pool.SchemaVersion == WorkPoolCurrentSchemaVersion:
		return nil
	case pool.SchemaVersion > WorkPoolCurrentSchemaVersion:
		return fmt.Errorf("unsupported workpool schema_version %d; current is %d", pool.SchemaVersion, WorkPoolCurrentSchemaVersion)
	default:
		return fmt.Errorf("unsupported workpool schema_version %d", pool.SchemaVersion)
	}
}

func normalizeTaskSchemaVersion(task *WorkTask) error {
	switch {
	case task.SchemaVersion == 0:
		task.SchemaVersion = WorkPoolCurrentSchemaVersion
		return nil
	case task.SchemaVersion == WorkPoolCurrentSchemaVersion:
		return nil
	case task.SchemaVersion > WorkPoolCurrentSchemaVersion:
		return fmt.Errorf("unsupported workpool schema_version %d; current is %d", task.SchemaVersion, WorkPoolCurrentSchemaVersion)
	default:
		return fmt.Errorf("unsupported workpool schema_version %d", task.SchemaVersion)
	}
}

func timestampNow() string {
	return workPoolNow().UTC().Format(time.RFC3339Nano)
}

var workPoolNow = time.Now
