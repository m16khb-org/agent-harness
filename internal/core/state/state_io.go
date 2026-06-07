package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/state/statepath"
)

func StateDir() string {
	return statepath.Dir()
}

func NormalizeStateKey(key string) (string, error) {
	return statepath.NormalizeKey(key)
}

func statePath(dir, key string) string {
	return statepath.Path(dir, key)
}

func StateWrite(key, content string) (StateResult, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return StateResult{OK: false, StateDir: StateDir()}, err
	}
	dir := StateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return StateResult{OK: false, StateDir: dir}, err
	}
	record := StateRecord{
		SchemaVersion: StateCurrentSchemaVersion,
		Key:           key,
		Content:       content,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Bytes:         len([]byte(content)),
	}
	path, err := writeStateRecord(dir, key, record)
	if err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	return StateResult{OK: true, StateDir: dir, Path: path, Record: record}, nil
}

func StateRead(key string) (StateResult, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return StateResult{OK: false, StateDir: StateDir()}, err
	}
	dir := StateDir()
	path := statePath(dir, key)
	b, err := os.ReadFile(path)
	if err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	var record StateRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	if record.Key != key {
		return StateResult{OK: false, StateDir: dir, Path: path}, fmt.Errorf("state key mismatch: file has %q", record.Key)
	}
	if record.Bytes != len([]byte(record.Content)) {
		return StateResult{OK: false, StateDir: dir, Path: path}, fmt.Errorf("state byte count mismatch for %q", key)
	}
	if record.SchemaVersion < 0 || record.SchemaVersion > StateCurrentSchemaVersion {
		return StateResult{OK: false, StateDir: dir, Path: path}, fmt.Errorf("unsupported state schema version %d for %q", record.SchemaVersion, key)
	}
	return StateResult{OK: true, StateDir: dir, Path: path, Record: record}, nil
}

func StateList() (StateListResult, error) {
	dir := StateDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return StateListResult{OK: true, StateDir: dir, Keys: []string{}, Records: []StateListEntry{}}, nil
	}
	if err != nil {
		return StateListResult{OK: false, StateDir: dir}, err
	}
	records := []StateListEntry{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		if _, err := NormalizeStateKey(key); err != nil {
			continue
		}
		result, err := StateRead(key)
		if err != nil {
			continue
		}
		records = append(records, StateListEntry{
			Key:           result.Record.Key,
			UpdatedAt:     result.Record.UpdatedAt,
			Bytes:         result.Record.Bytes,
			SchemaVersion: result.Record.SchemaVersion,
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Key < records[j].Key })
	keys := make([]string, 0, len(records))
	for _, record := range records {
		keys = append(keys, record.Key)
	}
	return StateListResult{OK: true, StateDir: dir, Keys: keys, Records: records}, nil
}

func writeStateRecord(dir, key string, record StateRecord) (string, error) {
	path := statePath(dir, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return path, err
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return path, err
	}
	tmp, err := os.CreateTemp(dir, "."+key+"-*.tmp")
	if err != nil {
		return path, err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(b); err != nil {
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
		return path, writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return path, err
	}
	return path, nil
}
