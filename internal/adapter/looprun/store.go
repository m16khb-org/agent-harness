package looprun

import (
	loopruncontract "agent-harness/internal/contract/looprun"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/adapter/outbound/state"
)

const loopBucket = "loop"

func StateRoot() string {
	return filepath.Join(state.StateDir(), "loop")
}

func openStore() (*sqlstore.DB, error) {
	return sqlstore.Open(StateRoot())
}

func ReadLoop(loopID string) (loopruncontract.LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	db, err := openStore()
	if err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	data, ok, err := db.Get(loopBucket, loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	if !ok {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, fmt.Errorf("loop %s: %w", loopID, fs.ErrNotExist)
	}
	return decodeLoop(loopID, data)
}

// ReadLoopExisting reads one existing loop without creating, repairing, or
// changing permissions on the loop store.
func ReadLoopExisting(loopID string) (loopruncontract.LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	data, ok, err := sqlstore.GetExisting(StateRoot(), loopBucket, loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	if !ok {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, fmt.Errorf("loop %s: %w", loopID, fs.ErrNotExist)
	}
	return decodeLoop(loopID, data)
}

func decodeLoop(loopID string, data []byte) (loopruncontract.LoopRun, error) {
	var loop loopruncontract.LoopRun
	if err := json.Unmarshal(data, &loop); err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	if loop.ID != loopID {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, fmt.Errorf("loop id mismatch: record has %q", loop.ID)
	}
	if err := normalizeLoopSchemaVersion(&loop); err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	loop.OK = true
	return loop, nil
}

func writeLoop(loop loopruncontract.LoopRun) (loopruncontract.LoopRun, error) {
	if _, err := normalizeLoopID(loop.ID); err != nil {
		loop.OK = false
		return loop, err
	}
	if err := normalizeLoopSchemaVersion(&loop); err != nil {
		loop.OK = false
		return loop, err
	}
	db, err := openStore()
	if err != nil {
		loop.OK = false
		return loop, err
	}
	loop.OK = true
	data, err := json.MarshalIndent(loop, "", "  ")
	if err != nil {
		loop.OK = false
		return loop, err
	}
	if err := db.Put(loopBucket, loop.ID, data); err != nil {
		loop.OK = false
		return loop, err
	}
	return loop, nil
}

func ListLoopIDs() ([]string, error) {
	ids, err := sqlstore.ListExisting(StateRoot(), loopBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func newLoopID(repo, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(name)))
	return "loop-" + hex.EncodeToString(sum[:])[:12]
}

func ResolveID(repo, name string) (string, error) {
	repo, err := normalizeRepo(repo)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	return newLoopID(repo, name), nil
}

func normalizeLoopID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("loop_id is required")
	}
	if !strings.HasPrefix(id, "loop-") || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid loop id %q", id)
	}
	return id, nil
}

func normalizeLoopSchemaVersion(loop *loopruncontract.LoopRun) error {
	switch {
	case loop.SchemaVersion == 0:
		loop.SchemaVersion = LoopRunCurrentSchemaVersion
		return nil
	case loop.SchemaVersion == LoopRunCurrentSchemaVersion:
		return nil
	case loop.SchemaVersion > LoopRunCurrentSchemaVersion:
		return fmt.Errorf("unsupported loop schema_version %d; current is %d", loop.SchemaVersion, LoopRunCurrentSchemaVersion)
	default:
		return fmt.Errorf("unsupported loop schema_version %d", loop.SchemaVersion)
	}
}

func timestampNow() string {
	return loopNow().UTC().Format(time.RFC3339Nano)
}

var loopNow = time.Now
