package workpool

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/state"
)

func StateRoot() string {
	return filepath.Join(state.StateDir(), "workpool")
}

func ReadPool(poolID string) (WorkPool, error) {
	poolID, err := normalizePoolID(poolID)
	if err != nil {
		return WorkPool{OK: false}, err
	}
	data, err := os.ReadFile(poolPath(poolID))
	if err != nil {
		return WorkPool{OK: false, ID: poolID}, err
	}
	var pool WorkPool
	if err := json.Unmarshal(data, &pool); err != nil {
		return WorkPool{OK: false, ID: poolID}, err
	}
	if pool.ID != poolID {
		return WorkPool{OK: false, ID: poolID}, fmt.Errorf("workpool id mismatch: file has %q", pool.ID)
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
	if err := os.MkdirAll(StateRoot(), 0o700); err != nil {
		pool.OK = false
		return pool, err
	}
	pool.OK = true
	if err := writeJSONFile(StateRoot(), poolPath(pool.ID), pool); err != nil {
		pool.OK = false
		return pool, err
	}
	return pool, nil
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
	data, err := os.ReadFile(taskPath(poolID, taskID))
	if err != nil {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, err
	}
	var task WorkTask
	if err := json.Unmarshal(data, &task); err != nil {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, err
	}
	if task.ID != taskID || task.PoolID != poolID {
		return WorkTask{OK: false, ID: taskID, PoolID: poolID}, fmt.Errorf("worktask id mismatch: file has %q/%q", task.PoolID, task.ID)
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
	dir := filepath.Join(StateRoot(), poolID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		task.OK = false
		return task, err
	}
	task.OK = true
	if err := writeJSONFile(dir, taskPath(poolID, task.ID), task); err != nil {
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

func taskFileIDs(poolID string) ([]string, error) {
	dir := filepath.Join(StateRoot(), poolID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "task-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(ids)
	return ids, nil
}

func newPoolID(repo, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(name)))
	return "wp-" + hex.EncodeToString(sum[:])[:12]
}

func newTaskID(seq int, title string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(title)))
	return fmt.Sprintf("task-%04d-%s", seq, hex.EncodeToString(sum[:])[:8])
}

func poolPath(poolID string) string {
	return filepath.Join(StateRoot(), poolID+".json")
}

func taskPath(poolID, taskID string) string {
	return filepath.Join(StateRoot(), poolID, taskID+".json")
}

func writeJSONFile(dir, path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".workpool-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(data); err != nil {
			return err
		}
		if _, err := tmp.Write([]byte{'\n'}); err != nil {
			return err
		}
		if err := tmp.Chmod(0o600); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
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
