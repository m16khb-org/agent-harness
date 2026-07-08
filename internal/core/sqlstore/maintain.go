package sqlstore

import (
	"fmt"
	"os"
	"path/filepath"
)

// MaintainResult reports one directory's store maintenance pass.
type MaintainResult struct {
	Dir              string   `json:"dir"`
	WALBytesBefore   int64    `json:"wal_bytes_before"`
	WALBytesAfter    int64    `json:"wal_bytes_after"`
	Checkpointed     bool     `json:"checkpointed"`
	PermissionsFixed []string `json:"permissions_fixed,omitempty"`
}

// Maintain runs the store's periodic maintenance: it truncates the data
// database's WAL via PRAGMA wal_checkpoint(TRUNCATE) and re-asserts 0600 on
// every store file and sidecar. It is safe to run concurrently with readers
// and writers; when a writer holds the WAL busy the checkpoint is skipped for
// this pass (Checkpointed=false) instead of failing.
func (d *DB) Maintain() (MaintainResult, error) {
	result := MaintainResult{Dir: d.dir}
	walPath := filepath.Join(d.dir, dataDBFile+"-wal")
	if info, err := os.Stat(walPath); err == nil {
		result.WALBytesBefore = info.Size()
	}
	var busy, logFrames, checkpointed int
	if err := d.data.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return result, fmt.Errorf("sqlstore maintain checkpoint %s: %w", d.dir, err)
	}
	result.Checkpointed = busy == 0
	if info, err := os.Stat(walPath); err == nil {
		result.WALBytesAfter = info.Size()
	}
	// Re-assert private permissions on every store file. Sidecar modes are not
	// reliably inherited from the database file in the wild (0644 sidecars have
	// been observed under umask 022), so repair rather than trust.
	for _, base := range []string{dataDBFile, spanDBFile} {
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			path := filepath.Join(d.dir, base+suffix)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Mode().Perm() == 0o600 {
				continue
			}
			if err := os.Chmod(path, 0o600); err != nil {
				return result, fmt.Errorf("sqlstore maintain chmod %s: %w", path, err)
			}
			result.PermissionsFixed = append(result.PermissionsFixed, base+suffix)
		}
	}
	return result, nil
}
