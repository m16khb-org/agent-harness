package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"time"

	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/core/state/statepath"
	statedomain "agent-harness/internal/domain/state"
)

// stateBucket is the sqlstore bucket holding one row per state key.
const stateBucket = "state"

func StateDir() string {
	return statepath.Dir()
}

func NormalizeStateKey(key string) (string, error) {
	return statepath.NormalizeKey(key)
}

// statePath returns the record's stable identifier path. Records live as rows
// in the state directory's sqlite database; the legacy <dir>/<key>.json shape
// is kept as the Path field's identifier so CLI/MCP output stays addressable
// per key rather than pointing every record at the same database file.
func statePath(dir, key string) string {
	return statepath.Path(dir, key)
}

func openStateDB(dir string) (*sqlstore.DB, error) {
	return sqlstore.Open(dir)
}

func StateWrite(key, content string) (StateResult, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return StateResult{OK: false, StateDir: StateDir()}, err
	}
	dir := StateDir()
	var result StateResult
	err = withStateLock(context.Background(), dir, key, func(context.Context) error {
		record := statecontract.RecordEnvelope{
			SchemaVersion: StateCurrentSchemaVersion,
			Key:           key,
			Content:       content,
			UpdatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
			Bytes:         len([]byte(content)),
		}
		path, writeErr := writeStateRecord(dir, key, record)
		result = StateResult{
			OK:       writeErr == nil,
			StateDir: dir,
			Path:     path,
			Record:   record,
		}
		return writeErr
	})
	if result.OK || result.StateDir != "" {
		return result, err
	}
	return StateResult{OK: false, StateDir: dir}, err
}

func StateRead(key string) (StateResult, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return StateResult{OK: false, StateDir: StateDir()}, err
	}
	dir := StateDir()
	path := statePath(dir, key)
	b, ok, err := sqlstore.GetExisting(dir, stateBucket, key)
	if err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	if !ok {
		// A *PathError keeps os.IsNotExist working for callers (StateUpdate's
		// missing-key tolerance) alongside errors.Is(err, fs.ErrNotExist).
		return StateResult{OK: false, StateDir: dir, Path: path}, &fs.PathError{Op: "read", Path: path, Err: fs.ErrNotExist}
	}
	record, err := decodeStateRecord(key, b)
	if err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	return StateResult{OK: true, StateDir: dir, Path: path, Record: record}, nil
}

func decodeStateRecord(key string, data []byte) (statecontract.RecordEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record statecontract.RecordEnvelope
	if err := decoder.Decode(&record); err != nil {
		return statecontract.RecordEnvelope{}, statecontract.Invalid("")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return statecontract.RecordEnvelope{}, statecontract.Invalid("")
	}
	if err := statedomain.ValidateRecord(key, record); err != nil {
		return statecontract.RecordEnvelope{}, statecontract.Invalid("")
	}
	return record, nil
}

func StateList() (StateListResult, error) {
	dir := StateDir()
	db, err := openStateDB(dir)
	if err != nil {
		return StateListResult{OK: false, StateDir: dir}, err
	}
	keys, err := db.List(stateBucket)
	if err != nil {
		return StateListResult{OK: false, StateDir: dir}, err
	}
	records := []StateListEntry{}
	for _, key := range keys {
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
	listKeys := make([]string, 0, len(records))
	for _, record := range records {
		listKeys = append(listKeys, record.Key)
	}
	return StateListResult{OK: true, StateDir: dir, Keys: listKeys, Records: records}, nil
}

// WriteStateRecord persists record under key in dir's state database, under
// the per-directory span lock, for callers that must write a record to a
// dir other than StateDir() (e.g. the self-augment snapshot writer). It is the
// locked equivalent of a raw writeStateRecord to that directory.
func WriteStateRecord(dir, key string, record statecontract.RecordEnvelope) (string, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return "", err
	}
	// Mirror the StateRead invariant (record.Key must match the file key): this is
	// the first writer that accepts a caller-built record alongside a separate key
	// arg, so guard against persisting a record StateRead would later reject.
	if record.Key != "" && record.Key != key {
		return "", fmt.Errorf("state record key %q does not match write key %q", record.Key, key)
	}
	var path string
	err = withStateLock(context.Background(), dir, key, func(context.Context) error {
		var werr error
		path, werr = writeStateRecord(dir, key, record)
		return werr
	})
	return path, err
}

func writeStateRecord(dir, key string, record statecontract.RecordEnvelope) (string, error) {
	path := statePath(dir, key)
	if err := statedomain.ValidateRecord(key, record); err != nil {
		return path, err
	}
	db, err := openStateDB(dir)
	if err != nil {
		return path, err
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return path, err
	}
	if err := db.Put(stateBucket, key, append(b, '\n')); err != nil {
		return path, err
	}
	return path, nil
}

// deleteStateRecord removes the record for key from dir's state database.
// Deleting an absent record is not an error.
func deleteStateRecord(dir, key string) error {
	db, err := openStateDB(dir)
	if err != nil {
		return err
	}
	return db.Delete(stateBucket, key)
}

// StateDelete removes the record for key from the default state directory.
// Deleting an absent record is not an error.
func StateDelete(key string) error {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return err
	}
	return deleteStateRecord(StateDir(), key)
}
